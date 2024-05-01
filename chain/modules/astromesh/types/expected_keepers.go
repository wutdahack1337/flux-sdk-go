package types

import (
	"context"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/FluxNFTLabs/sdk-go/chain/modules/evm/evmone"
	evmtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/evm/types"
	svmtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// BankKeeper defines the contract needed to be fulfilled for banking and supply dependencies.
type BankKeeper interface {
	Send(goCtx context.Context, msg *banktypes.MsgSend) (*banktypes.MsgSendResponse, error)

	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
	GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins
	SpendableCoins(ctx context.Context, addr sdk.AccAddress) sdk.Coins

	SendCoins(ctx context.Context, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	MintCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
	BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
	GetDenomMetaData(ctx context.Context, denom string) (banktypes.Metadata, bool)
	SetDenomMetaData(ctx context.Context, denomMetaData banktypes.Metadata)
}

// AccountKeeper defines the contract required for account APIs.
type AccountKeeper interface {
	GetModuleAddress(name string) sdk.AccAddress
	GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
}

type WasmViewKeeper interface {
	wasmtypes.ViewKeeper
}

type WasmOpsKeeper interface {
	wasmtypes.ContractOpsKeeper
}

type EvmKeeper interface {
	DefaultEVMContext(
		ctx sdk.Context,
		sender evmone.Address,
	) evmone.HostContext

	DeployWithDeterministicAddress(
		ctx sdk.Context,
		hostContext evmone.HostContext,
		compiledCode []byte,
		constructorCallData []byte,
		inputValue evmone.Hash,
		sender evmone.Address,
		contractAddress evmone.Address,
	) error

	ExecuteBytecode(
		ctx sdk.Context,
		hostContext evmone.HostContext,
		callKind evmone.CallKind,
		compiledCode []byte,
		callData []byte,
		inputAmount evmone.Hash,
		sender evmone.Address,
		contractAddress evmone.Address,
	) (evmone.Result, error)

	ExecuteContractByAddress(
		ctx sdk.Context,
		sender sdk.AccAddress,
		contractAddress []byte,
		calldata []byte,
		inputAmount []byte,
	) (evmone.Result, error)

	KVGetAccount(ctx context.Context, addr []byte) ([]byte, bool)
	KVSetAccount(ctx context.Context, addr []byte, ethBalance []byte)
	ContractQuery(goCtx context.Context, req *evmtypes.ContractQueryRequest) (res *evmtypes.ContractQueryResponse, err error)
}

type SvmKeeper interface {
	KVGetAccount(ctx context.Context, accAddr []byte) (*svmtypes.Account, bool)
	KVSetAccount(ctx context.Context, account *svmtypes.Account)
	SvmExecute(ctx sdk.Context, msg *svmtypes.MsgTransaction) (*svmtypes.MsgTransactionResponse, error)
}
