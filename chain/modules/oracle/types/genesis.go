package types

import sdkmath "cosmossdk.io/math"

// ValidateGenesis checks that the given genesis state has no integrity issues
func ValidateGenesis(data GenesisState) error {
	return nil
}

// DefaultGenesisState - Returns a default genesis state
func DefaultGenesisState() *GenesisState {
	return &GenesisState{
		SimpleEntries: []*SimpleEntry{
			{Symbol: "BTC", Decimal: 18, Value: sdkmath.NewIntFromUint64(71000), Timestamp: 0},
			{Symbol: "ETH", Decimal: 18, Value: sdkmath.NewIntFromUint64(192), Timestamp: 0},
			{Symbol: "SOL", Decimal: 18, Value: sdkmath.NewIntFromUint64(71000), Timestamp: 0},
			{Symbol: "LUX", Decimal: 18, Value: sdkmath.NewIntFromUint64(1), Timestamp: 0},
		},
		AuthorizedSimpleEntryAddresses: []string{
			"lux10tq6q4p67prfmhmzmdwg7zwx66v0gpfdygrr8z", // signer1 addr
			"lux1kmmz47pr8h46wcyxw8h3k8s85x0ncykqp0xmgj", // signer2 addr
		},
	}
}
