package main

import (
	"context"
	"fmt"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/goccy/go-json"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/FluxNFTLabs/sdk-go/chain/stream/types"
)

func main() {
	network := common.LoadNetwork("local", "")
	cc, err := grpc.Dial(network.ChainGrpcEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer cc.Close()
	if err != nil {
		panic(err)
	}
	client := types.NewQueryClient(cc)

	stream, err := client.StreamEvents(context.Background(), &types.EventsRequest{
		Modules:   []string{"fnft"},
		TmQueries: []string{"block", "block_results", "validators"},
	})
	if err != nil {
		panic(err)
	}

	for {
		res, err := stream.Recv()
		if err != nil {
			panic(err)
		}
		fmt.Println("===================", res.Height, res.Time)
		for i, module := range res.Modules {
			fmt.Println(module)
			for _, eventOp := range res.Events[i].EventOps {
				fmt.Println(eventOp)
			}
		}

		for i, query := range res.TmQueries {
			bz := []byte(res.TmData[i])
			fmt.Println(query)
			switch query {
			case "block":
				var block coretypes.ResultBlock
				err = json.Unmarshal(bz, &block)
				if err != nil {
					panic(err)
				}
				fmt.Println(block.Block.Height)
			case "block_results":
				var blockResults coretypes.ResultBlockResults
				err = json.Unmarshal(bz, &blockResults)
				if err != nil {
					panic(err)
				}
				fmt.Println(blockResults.Height)
			case "validators":
				fmt.Println(string(bz))
			}
		}
	}
}
