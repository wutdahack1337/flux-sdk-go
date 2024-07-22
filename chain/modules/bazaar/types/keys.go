package types

import sdk "github.com/cosmos/cosmos-sdk/types"

const (
	// ModuleName defines the name of the nft module
	ModuleName = "bazaar"

	// StoreKey is the default store key for nft
	StoreKey = ModuleName

	// RouterKey is the message route for nft
	RouterKey = ModuleName
)

var ProductCommissionMultisigAcc sdk.AccAddress
