package types

import (
	"github.com/gagliardetto/solana-go"
)

const (
	// ModuleName defines the name of the astromesh module
	ModuleName = "astromesh"

	// StoreKey is the default store key for astromesh
	StoreKey = ModuleName

	// RouterKey is the message route for astromesh
	RouterKey = ModuleName
)

var (
	// generate completely non key-pair account by using PDA where the program ID owner is also off-curve
	// just to make sure it couldn't be signed by any external accounts/any PDA
	SvmMintAuthority solana.PublicKey
	SvmPoolAuthority solana.PublicKey
)

func init() {
	authority, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("MintAuthority")},
		solana.MustPublicKeyFromBase58("Astromesh1111111111111111111111111111111111"),
	)
	if err != nil {
		panic(err)
	}

	SvmMintAuthority = authority

	poolAuthority, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("PoolAuthority")},
		solana.MustPublicKeyFromBase58("Astromesh1111111111111111111111111111111111"),
	)
	if err != nil {
		panic(err)
	}
	SvmPoolAuthority = poolAuthority
}
