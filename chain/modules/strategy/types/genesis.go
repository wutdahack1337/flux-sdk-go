package types

// ValidateGenesis checks that the given genesis state has no integrity issues
func ValidateGenesis(data GenesisState) error {
	return nil
}

// DefaultGenesisState - Returns a default genesis state
func DefaultGenesisState() *GenesisState {
	return &GenesisState{}
}
