package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/FluxNFTLabs/sdk-go/chain/eventstream/types"
	"github.com/FluxNFTLabs/sdk-go/client/common"
)

func main() {
	network := common.LoadNetwork("local", "")
	cc, err := grpc.Dial(network.ChainGrpcEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer cc.Close()
	if err != nil {
		panic(err)
	}
	client := types.NewQueryClient(cc)

	res, err := client.GetEvents(context.Background(), &types.EventsRequest{
		Height:    80,
		Modules:   []string{"astromesh"},
		TmQueries: []string{"block", "block_results", "validators"},
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(res)
}
