package main

import (
	"context"
	"fmt"
	astromeshtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	cfg := sdk.GetConfig()
	chaintypes.SetBech32Prefixes(cfg)
	chaintypes.SetBip44CoinType(cfg)
	cfg.Seal()

	// init grpc connection
	cc, err := grpc.Dial("localhost:9900", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	// init query client
	astromeshClient := astromeshtypes.NewQueryClient(cc)

	// query
	acc := sdk.MustAccAddressFromBech32("lux1cml96vmptgw99syqrrz8az79xer2pcgp209sv4")
	req := &astromeshtypes.FISQueryRequest{Instructions: []*astromeshtypes.FISQueryInstruction{
		{
			Plane:   astromeshtypes.Plane_COSMOS,
			Action:  astromeshtypes.QueryAction_COSMOS_BANK_BALANCE,
			Address: acc.Bytes(),
			Input: [][]byte{
				[]byte("lux"),
			},
		},
	}}
	res, err := astromeshClient.FISQuery(context.Background(), req)
	if err != nil {
		panic(err)
	}

	var balance sdk.Coin
	err = proto.Unmarshal(res.InstructionResponses[0].Output, &balance)
	if err != nil {
		panic(err)
	}

	fmt.Println(balance)
}
