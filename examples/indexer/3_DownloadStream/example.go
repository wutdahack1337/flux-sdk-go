package main

import (
	"context"
	"fmt"
	types "github.com/FluxNFTLabs/sdk-go/chain/indexer/media"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"time"
)

func main() {
	cc, err := grpc.Dial("localhost:4444", grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer cc.Close()
	if err != nil {
		panic(err)
	}
	client := types.NewAPIClient(cc)

	// init upload client
	dc, err := client.Download(context.Background(), &types.DownloadRequest{Path: "series_0_1_product.mp3"})
	if err != nil {
		panic(err)
	}

	// buffer chunks with size = 3
	chunkCh := make(chan []byte, 3)
	go mockPlayer(chunkCh)
	for {
		msg, err := dc.Recv()
		if err != nil {
			break
		}
		switch data := msg.Content.(type) {
		case *types.StreamMsg_Metadata:
			fmt.Println(data)
		case *types.StreamMsg_MediaChunk:
			chunkCh <- data.MediaChunk
		}
	}

	err = dc.CloseSend()
	if err != nil {
		panic(err)
	}
}

func mockPlayer(chunkCh chan []byte) {
	for {
		chunk, ok := <-chunkCh
		if !ok {
			break
		}
		// mock play the segment
		time.Sleep(time.Second * 1)
		fmt.Println("playing chunk with size", len(chunk))
	}
	fmt.Println("finished playing the media")
}
