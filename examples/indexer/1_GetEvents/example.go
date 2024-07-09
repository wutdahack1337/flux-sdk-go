package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/FluxNFTLabs/sdk-go/chain/stream/types"
)

func main() {
	cc, err := grpc.Dial(network.ChainGrpcEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer cc.Close()
	if err != nil {
		panic(err)
	}
	client := types.NewQueryClient(cc)

	res, err := client.GetEvents(context.Background(), &types.EventsRequest{
		Height:    67,
		Modules:   []string{"fnft"},
		TmQueries: []string{"block", "block_results", "validators"},
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(res)
}
