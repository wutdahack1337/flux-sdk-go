package main

import (
	"context"
	"fmt"

	"cosmossdk.io/collections"

	astromeshtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	network := common.LoadNetwork("local", "")
	cfg := sdk.GetConfig()
	chaintypes.SetBech32Prefixes(cfg)
	chaintypes.SetBip44CoinType(cfg)
	cfg.Seal()

	// init grpc connection
	cc, err := grpc.Dial(network.ChainGrpcEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	// init client ctx
	clientCtx, _, err := chaintypes.NewClientContext(
		network.ChainId,
		"",
		nil,
	)
	if err != nil {
		panic(err)
	}
	clientCtx = clientCtx.WithGRPCClient(cc)

	// init query client
	astromeshClient := astromeshtypes.NewQueryClient(cc)

	// query
	keypairCodec := collections.PairKeyCodec(sdk.AccAddressKey, collections.StringKey)
	addr := sdk.MustAccAddressFromBech32("lux1jcltmuhplrdcwp7stlr4hlhlhgd4htqhu86cqx")
	key, _ := collections.EncodeKeyWithPrefix(banktypes.BalancesPrefix, keypairCodec, collections.Join(addr, "lux"))
	fisReq := &astromeshtypes.FISQueryRequest{Instructions: []*astromeshtypes.FISQueryInstruction{
		{
			Plane:   astromeshtypes.Plane_COSMOS,
			Action:  astromeshtypes.QueryAction_COSMOS_KVSTORE,
			Address: []byte{},
			Input: [][]byte{
				[]byte("bank"),
				key,
			},
		},
	}}
	res, err := astromeshClient.FISQuery(context.Background(), fisReq)
	if err != nil {
		fmt.Println(res, err)
	}

	for _, ixRes := range res.InstructionResponses {
		for _, obj := range ixRes.Output {
			fmt.Println(string(obj))
		}
	}
}
