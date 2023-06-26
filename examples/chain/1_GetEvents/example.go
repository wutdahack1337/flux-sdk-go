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

	res, err := client.GetEvents(context.Background(), &types.GetEventsRequest{
		Height:  652,
		Modules: []string{"fnft"},
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(res)
}
