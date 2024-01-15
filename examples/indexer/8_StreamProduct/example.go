package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	types "github.com/FluxNFTLabs/sdk-go/chain/indexer/bazaar"
)

func main() {
	cc, err := grpc.Dial("localhost:4450", grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer cc.Close()
	if err != nil {
		panic(err)
	}
	client := types.NewAPIClient(cc)

	stream, err := client.StreamProducts(context.Background(), &types.ProductsRequest{
		ClassId:   "series",
		Id:        "",
		ProductId: "",
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
