package types

import "cosmossdk.io/errors"

var (
	ErrUnsupportedAction = errors.Register(ModuleName, 1, "unsupported action")
)
