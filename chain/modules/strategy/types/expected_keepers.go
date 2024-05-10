package types

import (
	"context"

	"github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"
)

type AstromeshKeeper interface {
	FISTransaction(goCtx context.Context, msg *types.MsgFISTransaction) (*types.MsgFISTransactionResponse, error)
	FISQuery(goCtx context.Context, msg *types.FISQueryRequest) (*types.FISQueryResponse, error)
}
