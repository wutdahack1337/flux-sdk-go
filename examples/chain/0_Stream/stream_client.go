package main

import (
	"context"
	"fmt"
	fnfttypes "github.com/FluxNFTLabs/sdk-go/chain/fnft/types"
	"github.com/cosmos/gogoproto/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/FluxNFTLabs/sdk-go/chain/stream/types"
)

func main() {
	cc, err := grpc.Dial("localhost:9999", grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer cc.Close()
	if err != nil {
		panic(err)
	}
	client := types.NewChainStreamClient(cc)

	stream, err := client.ChainStream(context.Background(), &types.ChainStreamRequest{
		Modules: []string{"fnft"},
	})
	if err != nil {
		panic(err)
	}

	for {
		res, err := stream.Recv()
		if err != nil {
			panic(err)
		}
		fmt.Println("===================", res.BlockHeight)
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
	}
}
