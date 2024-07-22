package types

import (
	"context"

	"github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type AstromeshKeeper interface {
	FISTransaction(goCtx context.Context, msg *types.MsgFISTransaction) (*types.MsgFISTransactionResponse, error)
	FISQuery(goCtx context.Context, msg *types.FISQueryRequest) (*types.FISQueryResponse, error)
}

type AccountKeeper interface {
	GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
}
