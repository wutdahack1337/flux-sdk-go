package main

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	types "github.com/FluxNFTLabs/sdk-go/chain/indexer/campclash"
)

func main() {
	cc, err := grpc.Dial("localhost:4462", grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer cc.Close()
	if err != nil {
		panic(err)
	}
	client := types.NewCampclashQueryClient(cc)

	stream, err := client.PushUserActivity(context.Background())
	if err != nil {
		panic(err)
	}

	for i := 0; i < 3; i++ {
		err = stream.Send(&types.PushUserActivityRequest{
			Address: "abc",
			Url:     "/home",
			Action:  types.Action_OPEN_PAGE,
		})
		if err != nil {
			panic(err)
		}
		time.Sleep(1 * time.Second)
	}

	stream.CloseSend()
}
