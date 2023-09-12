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
		if len(fnft.ClassId) == 0 {
			return ErrEmptyClassID
		}
		if len(fnft.Id) == 0 {
			return ErrEmptyNFTID
		}
		if _, err := sdk.AccAddressFromBech32(fnft.Owner); err != nil {
			return err
		}
	}
	for _, holder := range data.Holders {
		if len(holder.ClassId) == 0 {
			return ErrEmptyClassID
		}
		if len(holder.Id) == 0 {
			return ErrEmptyNFTID
		}
		if _, err := sdk.AccAddressFromBech32(holder.Address); err != nil {
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
				Url:         "https://flux.network",
			},
			{
				Id:          "livestream",
				Name:        "livestream",
				Description: "Livestream NFT",
				Url:         "https://flux.network",
			},
			{
				Id:          "music",
				Name:        "music",
				Description: "Music NFT",
				Url:         "https://flux.network",
			},
			{
				Id:          "consuming",
				Name:        "consuming",
				Description: "Consuming NFT (ticket, book, game...)",
				Url:         "https://flux.network",
			},
		},
		Nfts: []*NFT{},
	}
}
