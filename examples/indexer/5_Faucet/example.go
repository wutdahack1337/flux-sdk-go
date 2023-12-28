package main

import (
	"context"
	"github.com/FluxNFTLabs/sdk-go/chain/indexer/web3gw"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	cc, err := grpc.Dial("localhost:4444", grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer cc.Close()
	if err != nil {
		panic(err)
	}
	web3gwClient := web3gw.NewAPIClient(cc)

	ctx := context.Background()
	_, err = web3gwClient.FaucetSend(ctx, &web3gw.FaucetSendRequest{Address: "lux1cml96vmptgw99syqrrz8az79xer2pcgp209sv4"})
	if err != nil {
		panic(err)
	}
}
