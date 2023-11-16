package types

import (
	"github.com/pkg/errors"
	"os"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/std"
	cosmostypes "github.com/cosmos/cosmos-sdk/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"

	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	authztypes "github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	evidencetypes "cosmossdk.io/x/evidence/types"
	feegranttypes "cosmossdk.io/x/feegrant"
	govv1types "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	paramproposaltypes "github.com/cosmos/cosmos-sdk/x/params/types/proposal"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	ibcfeetypes "github.com/cosmos/ibc-go/v8/modules/apps/29-fee/types"
	ibcapplicationtypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	ibccoretypes "github.com/cosmos/ibc-go/v8/modules/core/types"

	keyscodec "github.com/FluxNFTLabs/sdk-go/chain/crypto/codec"
	fnfttypes "github.com/FluxNFTLabs/sdk-go/chain/modules/fnft/types"
)

type EncodingConfig struct {
	InterfaceRegistry types.InterfaceRegistry
	Codec             codec.Codec
	TxConfig          client.TxConfig
}

func init() {
	config := sdk.GetConfig()
	SetBech32Prefixes(config)
	SetBip44CoinType(config)
}

func RegisterTypes() types.InterfaceRegistry {
	interfaceRegistry := types.NewInterfaceRegistry()
	keyscodec.RegisterInterfaces(interfaceRegistry)
	std.RegisterInterfaces(interfaceRegistry)

	RegisterInterfaces(interfaceRegistry)
	fnfttypes.RegisterInterfaces(interfaceRegistry)

	// more cosmos types
	authtypes.RegisterInterfaces(interfaceRegistry)
	authztypes.RegisterInterfaces(interfaceRegistry)
	vestingtypes.RegisterInterfaces(interfaceRegistry)
	banktypes.RegisterInterfaces(interfaceRegistry)
	crisistypes.RegisterInterfaces(interfaceRegistry)
	distributiontypes.RegisterInterfaces(interfaceRegistry)
	evidencetypes.RegisterInterfaces(interfaceRegistry)
	govtypes.RegisterInterfaces(interfaceRegistry)
	govv1types.RegisterInterfaces(interfaceRegistry)
	paramproposaltypes.RegisterInterfaces(interfaceRegistry)
	ibcapplicationtypes.RegisterInterfaces(interfaceRegistry)
	ibccoretypes.RegisterInterfaces(interfaceRegistry)
	ibcfeetypes.RegisterInterfaces(interfaceRegistry)
	slashingtypes.RegisterInterfaces(interfaceRegistry)
	stakingtypes.RegisterInterfaces(interfaceRegistry)
	upgradetypes.RegisterInterfaces(interfaceRegistry)
	feegranttypes.RegisterInterfaces(interfaceRegistry)

	return interfaceRegistry
}

// NewTxConfig initializes new Cosmos TxConfig with certain signModes enabled.
func NewTxConfig(signModes []signingtypes.SignMode) client.TxConfig {
	registry := RegisterTypes()
	marshaler := codec.NewProtoCodec(registry)
	return tx.NewTxConfig(marshaler, signModes)
}

func NewClientContext(
	chainId, fromSpec string, kb keyring.Keyring,
) (client.Context, error) {
	clientCtx := client.Context{}
	registry := RegisterTypes()

	codec := codec.NewProtoCodec(registry)
	encodingConfig := EncodingConfig{
		InterfaceRegistry: registry,
		Codec:             codec,
		TxConfig: NewTxConfig([]signingtypes.SignMode{
			signingtypes.SignMode_SIGN_MODE_DIRECT,
		}),
	}

	var keyInfo keyring.Record

	if kb != nil {
		addr, err := cosmostypes.AccAddressFromBech32(fromSpec)
		if err == nil {
			record, err := kb.KeyByAddress(addr)
			if err != nil {
				err = errors.Wrapf(err, "failed to load key info by address %s", addr.String())
				return clientCtx, err
			}
			keyInfo = *record
		} else {
			// failed to parse Bech32, is it a name?
			record, err := kb.Key(fromSpec)
			if err != nil {
				err = errors.Wrapf(err, "no key in keyring for name: %s", fromSpec)
				return clientCtx, err
			}
			keyInfo = *record
		}
	}

	clientCtx = newContext(
		chainId,
		encodingConfig,
		kb,
		keyInfo,
	)

	return clientCtx, nil
}

func newContext(
	chainId string,
	encodingConfig EncodingConfig,
	kb keyring.Keyring,
	keyInfo keyring.Record,
) client.Context {
	clientCtx := client.Context{
		ChainID:           chainId,
		Codec:             encodingConfig.Codec,
		InterfaceRegistry: encodingConfig.InterfaceRegistry,
		Output:            os.Stderr,
		OutputFormat:      "json",
		BroadcastMode:     "block",
		UseLedger:         false,
		Simulate:          true,
		GenerateOnly:      false,
		Offline:           false,
		SkipConfirm:       true,
		TxConfig:          encodingConfig.TxConfig,
		AccountRetriever:  authtypes.AccountRetriever{},
	}

	if keyInfo.PubKey != nil {
		address, err := keyInfo.GetAddress()
		if err != nil {
			panic(err)
		}
		clientCtx = clientCtx.WithKeyring(kb)
		clientCtx = clientCtx.WithFromAddress(address)
		clientCtx = clientCtx.WithFromName(keyInfo.Name)
		clientCtx = clientCtx.WithFrom(keyInfo.Name)
	}

	return clientCtx
}
