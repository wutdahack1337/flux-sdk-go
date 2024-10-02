package types

import (
	"encoding/hex"
	fmt "fmt"

	sdkmath "cosmossdk.io/math"
	astromeshtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/gagliardetto/solana-go"
)

var (
	CRON_MINIMUM_GAS_PRICE = sdkmath.NewIntFromUint64(500_000_000)
)

func ParseContractAddr(contract string, plane astromeshtypes.Plane) ([]byte, error) {
	switch plane {
	case astromeshtypes.Plane_WASM:
		return sdk.MustAccAddressFromBech32(contract), nil
	case astromeshtypes.Plane_EVM:
		evmAddr, err := hex.DecodeString(contract)
		if err != nil {
			return nil, fmt.Errorf("invalid hex string for EVM address: %s", contract)
		}
		return evmAddr, nil
	case astromeshtypes.Plane_SVM:
		svmAddr := solana.MustPublicKeyFromBase58(contract)
		return svmAddr[:], nil

	default:
		return nil, fmt.Errorf("unsupported plane: %s", plane.String())
	}
}
