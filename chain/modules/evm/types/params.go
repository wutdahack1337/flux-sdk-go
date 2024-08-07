package types

import (
	"fmt"
)

// default params
var (
	// configurable params
	DEFAULT_EVM_BLOCK_GAS_LIMIT = int64(7000000)
	DEFAULT_EVM_GAS_PRICE       = uint64(1)
	DEFAULT_EVM_BASE_FEE        = uint64(1)

	// non-configurable params
	DEFAULT_EVM_START_DEPTH = 1
)

func NewParams(evmBlockGasLimit int64, evmGasPrice, evmBaseFee uint64) Params {
	return Params{
		EvmBlockGasLimit: evmBlockGasLimit,
		EvmGasPrice:      evmGasPrice,
		EvmBaseFee:       evmBaseFee,
	}
}

func DefaultParams() Params {
	return Params{
		EvmBlockGasLimit: DEFAULT_EVM_BLOCK_GAS_LIMIT,
		EvmGasPrice:      DEFAULT_EVM_GAS_PRICE,
		EvmBaseFee:       DEFAULT_EVM_BASE_FEE,
	}
}

func (p Params) Validate() error {
	if p.EvmBlockGasLimit <= 0 {
		return fmt.Errorf("invalid evm block gas limit")
	}
	if p.EvmGasPrice <= 0 {
		return fmt.Errorf("invalid evm gas price")
	}
	if p.EvmBaseFee <= 0 {
		return fmt.Errorf("invalid evm base fee")
	}
	return nil
}
