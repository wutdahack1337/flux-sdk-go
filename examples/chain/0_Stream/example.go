package main

import (
	"context"
	"fmt"
	fnfttypes "github.com/FluxNFTLabs/sdk-go/chain/fnft/types"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cosmos/gogoproto/proto"
	"github.com/goccy/go-json"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/FluxNFTLabs/sdk-go/chain/stream/types"
)

func main() {
	cc, err := grpc.Dial("localhost:9900", grpc.WithTransportCredentials(insecure.NewCredentials()))
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
			for _, rawEvent := range res.Events[i].RawEvents {
				var nft fnfttypes.NFT
				err = proto.Unmarshal(rawEvent.Value, &nft)
				if err != nil {
					panic(err)
				}
				fmt.Println(nft)
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
