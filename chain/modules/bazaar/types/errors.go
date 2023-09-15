package types

import (
	"cosmossdk.io/errors"
)

var (
	ErrEmptyProductID                  = errors.Register(ModuleName, 1, "empty product id")
	ErrProductNotExists                = errors.Register(ModuleName, 2, "product not exists")
	ErrEmptyOfferings                  = errors.Register(ModuleName, 3, "empty offerings")
	ErrMismatchOfferingLength          = errors.Register(ModuleName, 4, "mismatch offering idx and quantity lengths")
	ErrInvalidOfferingURL              = errors.Register(ModuleName, 5, "offering URL will be generated on chain")
	ErrInvalidOfferingDenom            = errors.Register(ModuleName, 6, "invalid offering denom")
	ErrInvalidOfferingAmount           = errors.Register(ModuleName, 7, "invalid offering amount")
	ErrInvalidOfferingPurchaseCount    = errors.Register(ModuleName, 8, "cannot set purchase count from product creation tx")
	ErrInvalidCommissionPart           = errors.Register(ModuleName, 9, "invalid class commission part")
	ErrUnverifiedOfferingPurchaseCount = errors.Register(ModuleName, 10, "offering purchase count is capped at 10 for unverified product")
	ErrVerifierNotExists               = errors.Register(ModuleName, 11, "only verifier address can verify product")
)
