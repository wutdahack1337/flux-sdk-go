package main

import (
	"context"
	"fmt"
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

	res, err := client.GetEvents(context.Background(), &types.EventsRequest{
		Height:  15381,
		Modules: []string{"fnft"},
		TmQueries: []string{"block", "block_results", "validators"},
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(res)
}
