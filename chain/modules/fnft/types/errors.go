package types

import (
	"cosmossdk.io/errors"
)

var (
	ErrClassExists    = errors.Register(ModuleName, 1, "nft class already exists")
	ErrClassNotExists = errors.Register(ModuleName, 2, "nft class does not exist")
	ErrNFTExists      = errors.Register(ModuleName, 3, "nft already exists")
	ErrNFTNotExists   = errors.Register(ModuleName, 4, "nft does not exist")
	ErrEmptyClassID   = errors.Register(ModuleName, 5, "empty class id")
	ErrEmptyNFTID     = errors.Register(ModuleName, 6, "empty nft id")
	ErrAcceptedDenom  = errors.Register(ModuleName, 7, "invalid sponsorship denom")
	ErrHolderNotFound = errors.Register(ModuleName, 8, "holder not found")
	ErrInvalidShares  = errors.Register(ModuleName, 9, "invalid shares")
	ErrInvalidISO     = errors.Register(ModuleName, 10, "action not supported in this period")
	ErrISORestriction = errors.Register(ModuleName, 11, "action not supported in this period")
)
