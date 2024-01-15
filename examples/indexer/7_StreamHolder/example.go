package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	types "github.com/FluxNFTLabs/sdk-go/chain/indexer/fnft"
)

func main() {
	cc, err := grpc.Dial("localhost:4447", grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer cc.Close()
	if err != nil {
		panic(err)
	}
	client := types.NewAPIClient(cc)

	stream, err := client.StreamHolders(context.Background(), &types.HoldersRequest{
		ClassId: "series",
		Id:      "",
		Address: "",
	})
	if err != nil {
		panic(err)
	}

	for {
		res, err := stream.Recv()
		if err != nil {
			panic(err)
		}
		fmt.Println(res)
	}
}
