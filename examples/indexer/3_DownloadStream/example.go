package main

import (
	"context"
	"cosmossdk.io/math"
	"fmt"
	types "github.com/FluxNFTLabs/sdk-go/chain/indexer/media"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"sync"
	"time"
)

const GRPC_MSG_SIZE = 134217728 // 128MB

func main() {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(GRPC_MSG_SIZE),
			grpc.MaxCallSendMsgSize(GRPC_MSG_SIZE),
		),
	}
	cc, err := grpc.Dial("localhost:4444", opts...)
	defer cc.Close()
	if err != nil {
		panic(err)
	}
	client := types.NewAPIClient(cc)

	// init upload client
	dc, err := client.Download(context.Background())
	if err != nil {
		panic(err)
	}

	// chunk params
	var metadata *types.Metadata
	bufferSize := uint64(3)
	chunkCh := make(chan []byte, bufferSize)
	chunkIdx := uint64(0)
	wg := new(sync.WaitGroup)
	wg.Add(2)

	// downloader routine
	doneFetching := false
	go func() {
		defer wg.Done()
		for {
			msg, err := dc.Recv()
			if err != nil {
				fmt.Println(err)
				break
			}
			switch data := msg.Content.(type) {
			case *types.StreamMsg_Metadata:
				metadata = data.Metadata
			case *types.StreamMsg_MediaChunk:
				chunkCh <- data.MediaChunk
				chunkIdx += 1
				if chunkIdx == metadata.ChunkSize {
					doneFetching = true
				}
			}
			if doneFetching {
				break
			}
		}
		close(chunkCh)
	}()

	// player routine
	go func() {
		defer wg.Done()
		for {
			// query chunk upon start or have 1 chunk left
			if len(chunkCh) <= 1 {
				var count = bufferSize
				if metadata != nil {
					count = math.Min[uint64](metadata.ChunkSize-chunkIdx, bufferSize)
				}
				err = dc.Send(&types.DownloadRequest{
					Path:       "series_0_0_product.mov",
					ChunkIdx:   chunkIdx,
					ChunkCount: count,
				})
				if err != nil {
					break
				}
			}
			// wait to receive chunk and play
			chunk, ok := <-chunkCh
			if !ok {
				err = dc.CloseSend()
				break
			}
			time.Sleep(time.Second * 1)
			fmt.Println("playing chunk with size:", len(chunk))
		}
		fmt.Println("finished playing the media, err:", err)
	}()

	wg.Wait()

	err = dc.CloseSend()
	if err != nil {
		panic(err)
	}
}
