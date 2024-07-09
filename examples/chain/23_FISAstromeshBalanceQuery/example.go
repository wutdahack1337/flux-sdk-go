package main

import (
	"context"
	"fmt"
	astromeshtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	cfg := sdk.GetConfig()
	chaintypes.SetBech32Prefixes(cfg)
	chaintypes.SetBip44CoinType(cfg)
	cfg.Seal()

	// init grpc connection
	cc, err := grpc.Dial(network.ChainGrpcEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	// init query client
	astromeshClient := astromeshtypes.NewQueryClient(cc)

	// query
	req := &astromeshtypes.FISQueryRequest{Instructions: []*astromeshtypes.FISQueryInstruction{
		{
			Plane:   astromeshtypes.Plane_COSMOS,
			Action:  astromeshtypes.QueryAction_COSMOS_ASTROMESH_BALANCE,
			Address: []byte{},
			Input: [][]byte{
				[]byte("lux1cml96vmptgw99syqrrz8az79xer2pcgp209sv4"),
				[]byte("usdt"),
			},
		},
	}}
	res, err := astromeshClient.FISQuery(context.Background(), req)
	if err != nil {
		panic(err)
	}

	for _, ixRes := range res.InstructionResponses {
		for _, obj := range ixRes.Output {
			fmt.Println(string(obj))
		}
	}
}
