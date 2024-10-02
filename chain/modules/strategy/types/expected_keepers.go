package types

import (
	"context"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"
	svmtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type AstromeshKeeper interface {
	FISTransaction(goCtx context.Context, msg *types.MsgFISTransaction) (*types.MsgFISTransactionResponse, error)
	FISQuery(goCtx context.Context, msg *types.FISQueryRequest) (*types.FISQueryResponse, error)
}

type AccountKeeper interface {
	GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
}

type WasmKeeper interface {
	GetContractInfo(ctx context.Context, contractAddress sdk.AccAddress) *wasmtypes.ContractInfo
}

type SvmKeeper interface {
	GetAccountLinkBySvmAddr(ctx context.Context, svmAddr []byte) (*svmtypes.AccountLink, bool)
}
