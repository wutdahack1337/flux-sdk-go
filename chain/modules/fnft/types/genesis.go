package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ValidateGenesis checks that the given genesis state has no integrity issues
func ValidateGenesis(data GenesisState) error {
	for _, class := range data.Classes {
		if len(class.Id) == 0 {
			return ErrEmptyClassID
		}
	}
	for _, fnft := range data.Nfts {
		if len(fnft.Id) == 0 {
			return ErrEmptyNFTID
		}
		if _, err := sdk.AccAddressFromBech32(fnft.Owner); err != nil {
			return err
		}
	}
	return nil
}

// DefaultGenesisState - Returns a default genesis state
func DefaultGenesisState() *GenesisState {
	return &GenesisState{
		Classes: []*Class{
			{
				Id:          "series",
				Name:        "series",
				Description: "Series NFT",
				Uri:         "https://flux.network",
			},
			{
				Id:          "livestream",
				Name:        "livestream",
				Description: "Livestream NFT",
				Uri:         "https://flux.network",
			},
			{
				Id:          "music",
				Name:        "music",
				Description: "Music NFT",
				Uri:         "https://flux.network",
			},
			{
				Id:          "consuming",
				Name:        "consuming",
				Description: "Consuming NFT (ticket, book, game...)",
				Uri:         "https://flux.network",
			},
		},
		Nfts: []*NFT{},
	}
}
