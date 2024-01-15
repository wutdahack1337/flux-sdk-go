package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	types "github.com/FluxNFTLabs/sdk-go/chain/indexer/fnft"
)

func main() {
	cc, err := grpc.Dial("localhost:4454", grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer cc.Close()
	if err != nil {
		panic(err)
	}
	client := types.NewAPIClient(cc)

	stream, err := client.StreamNFTs(context.Background(), &types.NFTsRequest{
		ClassId: "series",
		Id:      "",
		Owner:   "lux1cml96vmptgw99syqrrz8az79xer2pcgp209sv4",
		Status:  "Failed",
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
