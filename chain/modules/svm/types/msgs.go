package types

import (
	"fmt"

	"github.com/cosmos/btcutil/base58"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

var _ sdk.Msg = &MsgTransaction{}

func (m *MsgTransaction) ValidateBasic() error {
	// parse signers and check their uniqueness
	cosmosSigners := make([]sdk.AccAddress, len(m.CosmosSigners))
	seenCosmosSigners := map[string]struct{}{}
	for i, s := range m.CosmosSigners {
		signer, err := sdk.AccAddressFromBech32(s)
		if err != nil {
			return fmt.Errorf("%d-th signer is not valid bech32 format", i)
		}

		if _, exist := seenCosmosSigners[string(signer)]; exist {
			return fmt.Errorf("%d-th signer is duplicated", i)
		}

		cosmosSigners[i] = signer
		seenCosmosSigners[string(signer)] = struct{}{}
	}

	if m.ComputeBudget == 0 {
		return fmt.Errorf("compute budget cannot be zero")
	}

	if len(m.Accounts) == 0 {
		return fmt.Errorf("tx accounts array cannot be empty")
	}

	if len(m.Instructions) == 0 {
		return fmt.Errorf("tx instructions array cannot be empty")
	}

	// don't allow duplicate tx accounts
	txAccountsMap := map[string]struct{}{}
	for _, account := range m.Accounts {
		if _, exist := txAccountsMap[account]; exist {
			return fmt.Errorf("duplicate account in tx account list %s", account)
		} else {
			txAccountsMap[account] = struct{}{}
		}
	}

	// verify ix account indexes
	signerMap := map[string]bool{}
	for _, ix := range m.Instructions {
		calleeIndexMap := map[uint32]uint32{}
		for idx, ixAccount := range ix.Accounts {
			if ixAccount.IdIndex > uint32(len(m.Accounts)) {
				return fmt.Errorf("ix account index out of range")
			}

			if ixAccount.CallerIndex > uint32(len(m.Accounts)) {
				return fmt.Errorf("ix account caller index out of range")
			}

			if ixAccount.CalleeIndex > uint32(len(ix.Accounts)) {
				return fmt.Errorf("ix account callee index of range")
			}

			pubkey := m.Accounts[ixAccount.IdIndex]
			if ixAccount.IsSigner {
				signerMap[pubkey] = true
			}

			if _, exist := calleeIndexMap[ixAccount.IdIndex]; !exist {
				calleeIndexMap[ixAccount.IdIndex] = uint32(idx)
			}

			if ixAccount.CalleeIndex != calleeIndexMap[ixAccount.IdIndex] {
				return fmt.Errorf("callee index must be the first position of the account in this instruction")
			}
		}
	}

	// number of unique signers in ixs has to equal to number comsos signers
	if len(signerMap) != len(cosmosSigners) {
		return fmt.Errorf("number of signer in instructions must match cosmos signers (%d != %d)", len(signerMap), len(cosmosSigners))
	}

	for _, cosmosAddr := range cosmosSigners {
		expectedSvmPubkey := ethcrypto.Keccak256Hash(cosmosAddr[:])
		b58Pubkey := base58.Encode(expectedSvmPubkey[:])
		if _, exist := signerMap[b58Pubkey]; !exist {
			return fmt.Errorf("expected signers doesn't exist: %s", b58Pubkey)
		}
	}
	return nil
}

// This is for legacy version of cosmos sdk (< 0.5), for newer version, use the cosmos.v1.msg.signer option
func (m *MsgTransaction) GetSigners() (signers []sdk.AccAddress) {
	signers = make([]sdk.AccAddress, len(m.CosmosSigners))
	for i, s := range m.CosmosSigners {
		signer, _ := sdk.AccAddressFromBech32(s)
		signers[i] = signer
	}
	return signers
}
