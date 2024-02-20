package types

import "cosmossdk.io/errors"

var (
	ErrContractAddressCollision = errors.Register(ModuleName, 1, "Contract address exists")
	ErrNoDeployableCode         = errors.Register(ModuleName, 2, "No deployable code to store")
	ErrBytecodeExecution        = errors.Register(ModuleName, 3, "Bytecode execution error")
	ErrInvalidAccount           = errors.Register(ModuleName, 4, "Invalid account")
	ErrInvalidContract          = errors.Register(ModuleName, 5, "Invalid contract")
)
