package types

import "cosmossdk.io/errors"

var (
	ErrAccountNotExisted = errors.Register(ModuleName, 1, "Account not existed")
	ErrInvalidBase58     = errors.Register(ModuleName, 2, "Invalid base58 format")
)
