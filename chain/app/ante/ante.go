package ante

import (
	"context"
	"fmt"

	svmkeeper "github.com/FluxNFTLabs/sdk-go/chain/modules/svm/keeper"

	corestoretypes "cosmossdk.io/core/store"
	"cosmossdk.io/errors"
	storetypes "cosmossdk.io/store/types"
	circuitante "cosmossdk.io/x/circuit/ante"
	circuitkeeper "cosmossdk.io/x/circuit/keeper"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	svmante "github.com/FluxNFTLabs/sdk-go/chain/modules/svm/ante"
	log "github.com/InjectiveLabs/suplog"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/types/multisig"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	ibcante "github.com/cosmos/ibc-go/v8/modules/core/ante"
	ibckeeper "github.com/cosmos/ibc-go/v8/modules/core/keeper"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ethsecp256k1"
)

const (
	// TODO: Use this cost per byte through parameter or overriding NewConsumeGasForTxSizeDecorator
	// which currently defaults at 10, if intended
	// memoCostPerByte     sdk.Gas = 3
	ethSecp256k1VerifyCost uint64 = 21000
)

var (
	SvmDecoratorEnabled = true
)

// AccountKeeper defines an expected keeper interface for the auth module's AccountKeeper
type AccountKeeper interface {
	NewAccount(context.Context, sdk.AccountI) sdk.AccountI
	NewAccountWithAddress(ctx context.Context, addr sdk.AccAddress) sdk.AccountI

	GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
	GetAllAccounts(ctx context.Context) []sdk.AccountI
	SetAccount(ctx context.Context, acc sdk.AccountI)

	IterateAccounts(ctx context.Context, process func(sdk.AccountI) bool)

	ValidatePermissions(macc sdk.ModuleAccountI) error

	GetModuleAddress(moduleName string) sdk.AccAddress
	GetModuleAddressAndPermissions(moduleName string) (addr sdk.AccAddress, permissions []string)
	GetModuleAccountAndPermissions(ctx context.Context, moduleName string) (sdk.ModuleAccountI, []string)
	GetModuleAccount(ctx context.Context, moduleName string) sdk.ModuleAccountI
	SetModuleAccount(ctx context.Context, macc sdk.ModuleAccountI)

	authante.AccountKeeper
}

// BankKeeper defines an expected keeper interface for the bank module's Keeper
type BankKeeper interface {
	authtypes.BankKeeper
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
}

// FeegrantKeeper defines an expected keeper interface for the feegrant module's Keeper
type FeegrantKeeper interface {
	UseGrantedFees(ctx context.Context, granter, grantee sdk.AccAddress, fee sdk.Coins, msgs []sdk.Msg) error
}

type HandlerOptions struct {
	authante.HandlerOptions

	IBCKeeper             *ibckeeper.Keeper
	WasmConfig            *wasmtypes.WasmConfig
	WasmKeeper            *wasmkeeper.Keeper
	TXCounterStoreService corestoretypes.KVStoreService
	CircuitKeeper         *circuitkeeper.Keeper

	SvmKeeper *svmkeeper.Keeper
}

// NewAnteHandler constructor
func NewAnteHandler(options HandlerOptions) sdk.AnteHandler {
	return func(ctx sdk.Context, tx sdk.Tx, sim bool) (newCtx sdk.Context, err error) {
		var anteDecorators sdk.AnteHandler

		if options.AccountKeeper == nil {
			panic("account keeper is required for ante builder")
		}
		if options.BankKeeper == nil {
			panic("bank keeper is required for ante builder")
		}
		if options.SignModeHandler == nil {
			panic("sign mode handler is required for ante builder")
		}
		if options.WasmConfig == nil {
			panic("wasm config is required for ante builder")
		}
		if options.TXCounterStoreService == nil {
			panic("wasm store service is required for ante builder")
		}
		if options.CircuitKeeper == nil {
			panic("circuit keeper is required for ante builder")
		}
		if options.SvmKeeper == nil {
			panic("svm keeper is required for ante builder")
		}

		// web3 extension supporting EIP712 ante decorators
		txWithExtensions, ok := tx.(authante.HasExtensionOptionsTx)
		if ok {
			opts := txWithExtensions.GetExtensionOptions()
			if len(opts) > 0 {
				switch typeURL := opts[0].GetTypeUrl(); typeURL {
				case "/flux.types.v1beta1.ExtensionOptionsWeb3Tx":
					// handle as normal Cosmos SDK tx, except signature is checked for EIP712 representation
					switch tx.(type) {
					case sdk.Tx:
						anteDecorators = sdk.ChainAnteDecorators(
							authante.NewSetUpContextDecorator(),                                              // outermost AnteDecorator. SetUpContext must be called first
							wasmkeeper.NewLimitSimulationGasDecorator(options.WasmConfig.SimulationGasLimit), // after setup context to enforce limits early
							wasmkeeper.NewCountTXDecorator(options.TXCounterStoreService),
							wasmkeeper.NewGasRegisterDecorator(options.WasmKeeper.GetGasRegister()),
							circuitante.NewCircuitBreakerDecorator(options.CircuitKeeper),
							authante.NewValidateBasicDecorator(),
							authante.NewTxTimeoutHeightDecorator(),
							authante.NewValidateMemoDecorator(options.AccountKeeper),
							authante.NewConsumeGasForTxSizeDecorator(options.AccountKeeper),
							authante.NewSetPubKeyDecorator(options.AccountKeeper),
							authante.NewValidateSigCountDecorator(options.AccountKeeper),
							NewDeductFeeDecorator(options.AccountKeeper.(AccountKeeper), options.BankKeeper),
							authante.NewSigGasConsumeDecorator(options.AccountKeeper, DefaultSigVerificationGasConsumer),
							NewEip712SigVerificationDecorator(options.AccountKeeper.(AccountKeeper), options.SignModeHandler),
							svmante.NewSvmDecorator(options.SvmKeeper, SvmDecoratorEnabled),
							authante.NewIncrementSequenceDecorator(options.AccountKeeper),
							ibcante.NewRedundantRelayDecorator(options.IBCKeeper),
						)
					default:
						return ctx, errors.Wrapf(sdkerrors.ErrUnknownRequest, "invalid transaction type: %T", tx)
					}

				default:
					log.WithField("type_url", typeURL).Errorln("rejecting tx with unsupported extension option")
					return ctx, sdkerrors.ErrUnknownExtensionOptions
				}

				return anteDecorators(ctx, tx, sim)
			}
		}

		// cosmos tx ante decorators
		anteDecorators = sdk.ChainAnteDecorators(
			authante.NewSetUpContextDecorator(),                                              // outermost AnteDecorator. SetUpContext must be called first
			wasmkeeper.NewLimitSimulationGasDecorator(options.WasmConfig.SimulationGasLimit), // after setup context to enforce limits early
			wasmkeeper.NewCountTXDecorator(options.TXCounterStoreService),
			wasmkeeper.NewGasRegisterDecorator(options.WasmKeeper.GetGasRegister()),
			circuitante.NewCircuitBreakerDecorator(options.CircuitKeeper),
			authante.NewExtensionOptionsDecorator(options.ExtensionOptionChecker),
			authante.NewValidateBasicDecorator(),
			authante.NewTxTimeoutHeightDecorator(),
			authante.NewValidateMemoDecorator(options.AccountKeeper),
			authante.NewConsumeGasForTxSizeDecorator(options.AccountKeeper),
			authante.NewDeductFeeDecorator(options.AccountKeeper, options.BankKeeper, options.FeegrantKeeper, options.TxFeeChecker),
			authante.NewSetPubKeyDecorator(options.AccountKeeper), // SetPubKeyDecorator must be called before all signature verification decorators
			authante.NewValidateSigCountDecorator(options.AccountKeeper),
			authante.NewSigGasConsumeDecorator(options.AccountKeeper, options.SigGasConsumer),
			authante.NewSigVerificationDecorator(options.AccountKeeper, options.SignModeHandler),
			svmante.NewSvmDecorator(options.SvmKeeper, SvmDecoratorEnabled),
			authante.NewIncrementSequenceDecorator(options.AccountKeeper),
			ibcante.NewRedundantRelayDecorator(options.IBCKeeper),
		)

		return anteDecorators(ctx, tx, sim)
	}
}

var _ = DefaultSigVerificationGasConsumer

// DefaultSigVerificationGasConsumer is the default implementation of SignatureVerificationGasConsumer. It consumes gas
// for signature verification based upon the public key type. The cost is fetched from the given params and is matched
// by the concrete type.
func DefaultSigVerificationGasConsumer(
	meter storetypes.GasMeter, sig signing.SignatureV2, params authtypes.Params,
) error {
	pubkey := sig.PubKey
	switch pubkey := pubkey.(type) {
	case *ed25519.PubKey:
		meter.ConsumeGas(params.SigVerifyCostED25519, "ante verify: ed25519")
		return nil

	case *ethsecp256k1.PubKey:
		meter.ConsumeGas(params.SigVerifyCostSecp256k1, "ante verify: ethsecp256k1")
		return nil

	case multisig.PubKey:
		multisignature, ok := sig.Data.(*signing.MultiSignatureData)
		if !ok {
			return fmt.Errorf("expected %T, got, %T", &signing.MultiSignatureData{}, sig.Data)
		}
		err := ConsumeMultisignatureVerificationGas(meter, multisignature, pubkey, params, sig.Sequence)
		if err != nil {
			return err
		}
		return nil

	default:
		return errors.Wrapf(sdkerrors.ErrInvalidPubKey, "unrecognized public key type: %T", pubkey)
	}
}

// ConsumeMultisignatureVerificationGas consumes gas from a GasMeter for verifying a multisig pubkey signature
func ConsumeMultisignatureVerificationGas(
	meter storetypes.GasMeter, sig *signing.MultiSignatureData, pubkey multisig.PubKey,
	params authtypes.Params, accSeq uint64,
) error {

	size := sig.BitArray.Count()
	sigIndex := 0

	for i := 0; i < size; i++ {
		if !sig.BitArray.GetIndex(i) {
			continue
		}
		sigV2 := signing.SignatureV2{
			PubKey:   pubkey.GetPubKeys()[i],
			Data:     sig.Signatures[sigIndex],
			Sequence: accSeq,
		}
		err := DefaultSigVerificationGasConsumer(meter, sigV2, params)
		if err != nil {
			return err
		}
		sigIndex++
	}

	return nil
}
