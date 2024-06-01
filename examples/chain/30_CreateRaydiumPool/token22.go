package main

import (
	svmtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	"github.com/gagliardetto/solana-go"
)

type CreateNativeMint struct{}

var Sol22NativeMint = solana.MustPublicKeyFromBase58("9pan9bMn5HatX4EJdBwg9VgCa7Uz5HL8N1m5D3NdXejP")

func NewCreateNativeMintInstruction(
	fundingAccount solana.PublicKey,
	nativeMintAddress solana.PublicKey,
	systemProgram solana.PublicKey,
) *solana.GenericInstruction {
	return solana.NewInstruction(
		svmtypes.SplToken2022ProgramId,
		solana.AccountMetaSlice{
			{
				PublicKey:  fundingAccount,
				IsSigner:   true,
				IsWritable: true,
			},
			{
				PublicKey:  nativeMintAddress,
				IsSigner:   false,
				IsWritable: true,
			},
			{
				PublicKey:  systemProgram,
				IsSigner:   false,
				IsWritable: false,
			},
		},
		[]byte{31},
	)
}
