package types

import "github.com/ethereum/go-ethereum/crypto"

const (
	// ModuleName defines the name of the astromesh module
	ModuleName = "astromesh"

	// StoreKey is the default store key for astromesh
	StoreKey = ModuleName

	// RouterKey is the message route for astromesh
	RouterKey = ModuleName
)

var (
	SvmMintAuthority = crypto.Keccak256Hash([]byte(ModuleName))
)
