package ante

import (
	errorsmod "cosmossdk.io/errors"
	"fmt"
	"github.com/cosmos/btcutil/base58"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	"github.com/gagliardetto/solana-go"

	svmtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
)

type SvmDecorator struct {
	k         svmtypes.SvmKeeper
	isEnabled bool
}

func NewSvmDecorator(k svmtypes.SvmKeeper, isEnabled bool) SvmDecorator {
	return SvmDecorator{k: k, isEnabled: isEnabled}
}

func (svd SvmDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	if !svd.isEnabled {
		return next(ctx, tx, simulate)
	}

	svmTransactMsgExist := false
	msgTransactName := proto.MessageName(&svmtypes.MsgTransaction{})
	for _, msg := range tx.GetMsgs() {
		name := proto.MessageName(msg)
		if name == msgTransactName {
			svmTransactMsgExist = true
			break
		}
	}

	// continue on non-svm tx
	if !svmTransactMsgExist {
		return next(ctx, tx, simulate)
	}

	// throw error if svm tx has more than 1 msg
	if svmTransactMsgExist && len(tx.GetMsgs()) > 1 {
		return ctx, fmt.Errorf("svm transaction must have only one MsgTransaction")
	}

	// get signers, at this step signatures are already verified
	sigTx, ok := tx.(authsigning.Tx)
	if !ok {
		return ctx, errorsmod.Wrap(sdkerrors.ErrTxDecode, "invalid transaction type")
	}

	signers, err := sigTx.GetSigners()
	if err != nil {
		return ctx, err
	}

	signerMap := map[string]bool{}
	for _, signer := range signers {
		signerMap[string(signer)] = true
	}

	svmMsg, ok := tx.GetMsgs()[0].(*svmtypes.MsgTransaction)
	if !ok {
		return ctx, fmt.Errorf("failed to cast tx msg into *svmtypes.MsgTransaction")
	}

	// ensure sender links exist
	signerLinks := []*svmtypes.AccountLink{}
	for _, signer := range svmMsg.Signers {
		signerAcc := sdk.MustAccAddressFromBech32(signer)
		link, exist := svd.k.GetCosmosAccountLink(ctx, signerAcc.Bytes())
		if !exist {
			return ctx, fmt.Errorf("signer cosmos addr %s is not linked to any svm pubkey", signerAcc.String())
		}

		signerLinks = append(signerLinks, link)
	}

	for _, ix := range svmMsg.Instructions {
		// current instruction is create new account
		if svmMsg.Accounts[ix.ProgramIndex[0]] == svmtypes.SystemProgramId.String() && ix.Data[0] == 0 {
			valid := false
			for _, ixAcc := range ix.Accounts {
				if ixAcc.IsSigner {
					// the signer must be valid on-curve pubkey
					pubkey, err := solana.PublicKeyFromBase58(svmMsg.Accounts[ixAcc.CallerIndex])
					if err != nil {
						return ctx, fmt.Errorf("invalid signer pubkey: %s", svmMsg.Accounts[ixAcc.CallerIndex])
					}

					if !pubkey.IsOnCurve() {
						return ctx, fmt.Errorf("signer must be on curve: %s", svmMsg.Accounts[ixAcc.CallerIndex])
					}

					// verify sender link appears in signer map, always the 1st account
					sender := signerLinks[0]
					if base58.Encode(sender.To) == svmMsg.Accounts[ixAcc.CallerIndex] && signerMap[string(sender.From)] {
						valid = true
					}
				}
			}

			if !valid {
				return ctx, fmt.Errorf("sender cosmos account doesn't appear in signer list")
			}
		} else {
			// for all other instructions
			for _, ixAcc := range ix.Accounts {
				if ixAcc.IsSigner {
					// account link must exist
					link, exist := svd.k.GetSvmAccountLink(ctx, base58.Decode(svmMsg.Accounts[ixAcc.CallerIndex]))
					if !exist {
						return ctx, fmt.Errorf("ix account %s is not linked to any cosmos addr", svmMsg.Accounts[ixAcc.CallerIndex])
					}
					// account must appear in signer map
					if !signerMap[string(link.To)] {
						return ctx, fmt.Errorf("ix account %s linked cosmos addr %s doesn't appear in signer map", svmMsg.Accounts[ixAcc.CallerIndex], sdk.AccAddress(link.To).String())
					}
				}
			}
		}
	}

	return next(ctx, tx, simulate)
}
