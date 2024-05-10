package golana

import (
	"github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	"github.com/gagliardetto/solana-go"
)

func ToCosmosMsg(signer string, computeBudget uint64, tx *solana.Transaction) *types.MsgTransaction {
	pubkeys := []string{}
	for _, p := range tx.Message.AccountKeys {
		pubkeys = append(pubkeys, p.String())
	}

	ixs := []*types.Instruction{}
	for _, ix := range tx.Message.Instructions {
		fluxInstr := &types.Instruction{
			ProgramIndex: []uint32{uint32(ix.ProgramIDIndex)},
			Data:         ix.Data,
		}

		accounts, err := ix.ResolveInstructionAccounts(&tx.Message)
		if err != nil {
			panic(err)
		}

		for _, a := range accounts {
			fluxInstr.Accounts = append(fluxInstr.Accounts, &types.InstructionAccount{
				IdIndex:     uint32(positionOf(a.PublicKey, tx.Message.AccountKeys)),
				CallerIndex: uint32(positionOf(a.PublicKey, tx.Message.AccountKeys)),
				CalleeIndex: uint32(positionOfPubkeyInAccountMetas(a.PublicKey, accounts)),
				IsSigner:    a.IsSigner,
				IsWritable:  a.IsWritable,
			})
		}
		ixs = append(ixs, fluxInstr)
	}

	return &types.MsgTransaction{
		Sender:        signer,
		Accounts:      pubkeys,
		Instructions:  ixs,
		ComputeBudget: computeBudget,
	}
}

func positionOf(a solana.PublicKey, s []solana.PublicKey) int {
	for i, pk := range s {
		if pk.Equals(a) {
			return i
		}
	}
	return -1
}

func positionOfPubkeyInAccountMetas(a solana.PublicKey, metas []*solana.AccountMeta) int {
	for idx, meta := range metas {
		if meta.PublicKey.Equals(a) {
			return idx
		}
	}

	return -1
}
