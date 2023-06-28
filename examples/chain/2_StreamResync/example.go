package main

import (
	"context"
	"fmt"
	"github.com/FluxNFTLabs/sdk-go/chain/stream/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"strings"
	"sync"
	"time"
)

func main() {
	cc, err := grpc.Dial("localhost:9999", grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer cc.Close()
	if err != nil {
		panic(err)
	}
	client := types.NewChainStreamClient(cc)

	requestHeight := uint64(1)
	catchUp := false

	stream, err := client.EventsStream(context.Background(), &types.EventsRequest{
		Height:  requestHeight,
		Modules: []string{"fnft"},
	})
	if err != nil {
		panic(err)
	}

	// stream current and new blocks
	blockMap := map[uint64]*types.EventsResponse{}
	blockCh := make(chan *types.EventsResponse, 1000000)
	heightCh := make(chan uint64)
	mux := new(sync.RWMutex)
	go func() {
		for {
			res, err := stream.Recv()
			if err != nil {
				panic(err)
			}

			mux.Lock()
			blockMap[res.Height] = res
			mux.Unlock()

			fmt.Println("stream height", res.Height)

			if catchUp {
				heightCh <- res.Height
			}
		}
	}()

	// fetch past blocks until current height and return
	fetchSize := 10
	ctx := context.Background()
	go func() {
		nextHeight := requestHeight
		for {
			wg := new(sync.WaitGroup)
			wg.Add(fetchSize)
			for i := nextHeight; i < nextHeight+uint64(fetchSize); i++ {
				go func(height uint64) {
					defer wg.Done()

					res, err := client.GetEvents(ctx, &types.EventsRequest{Height: height, Modules: []string{"fnft"}})
					if err != nil && strings.Contains(err.Error(), "height doesn't exist") {
						catchUp = true
						return
					}

					mux.Lock()
					blockMap[res.Height] = res
					mux.Unlock()
				}(i)
				if catchUp {
					break
				}
			}
			if catchUp {
				break
			}
			wg.Wait()

			nextHeight = nextHeight + uint64(fetchSize)
			if catchUp {
				heightCh <- nextHeight
			} else {
				heightCh <- nextHeight - 1
			}
		}
	}()

	// output blocks into channel
	go func() {
		lastHeight := requestHeight
		for {
			select {
			case availableHeight := <-heightCh:
				for i := lastHeight; i <= availableHeight; i++ {
					mux.Lock()
					block := blockMap[i]
					if block != nil {
						blockCh <- block
						delete(blockMap, i)
					}
					mux.Unlock()
				}
			}
		}
	}()

	// process block
	go func() {
		for {
			select {
			case block := <-blockCh:
				fmt.Println("processed block", block)
				time.Sleep(time.Millisecond * 300)
			}
		}
	}()

	wg := new(sync.WaitGroup)
	wg.Add(1)
	wg.Wait()
}
