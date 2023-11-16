package main

import (
	sdkmath "cosmossdk.io/math"
	"fmt"
	bazaartypes "github.com/FluxNFTLabs/sdk-go/chain/modules/bazaar/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"os"

	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
)

func main() {
	network := common.LoadNetwork("local", "")
	tmClient, err := rpchttp.New(network.TmEndpoint, "/websocket")
	if err != nil {
		panic(err)
	}

	senderAddress, cosmosKeyring, err := chainclient.InitCosmosKeyring(
		os.Getenv("HOME")+"/.fluxd",
		"fluxd",
		"file",
		"user1",
		"",
		"", // keyring will be used if pk not provided
		false,
	)

	if err != nil {
		panic(err)
	}

	// initialize grpc client
	clientCtx, err := chaintypes.NewClientContext(
		network.ChainId,
		senderAddress.String(),
		cosmosKeyring,
	)
	if err != nil {
		fmt.Println(err)
	}
	clientCtx = clientCtx.WithNodeURI(network.TmEndpoint).WithClient(tmClient)

	// prepare tx msg
	msg := &bazaartypes.MsgCreateProduct{
		Sender:      senderAddress.String(),
		ClassId:     "series",
		Id:          "0",
		Title:       "Romance dawn",
		Description: "A random kid accidentally ate a fruit that disables swimming ability but now his body gains rubber properties",
		Offerings: []*bazaartypes.Offering{
			{Price: &sdk.Coin{
				Denom:  "ibc0xdAC17F958D2ee523a2206206994597C13D831ec7",
				Amount: sdkmath.NewIntFromUint64(2),
			}},
			{Price: &sdk.Coin{
				Denom:  "ibc0xdAC17F958D2ee523a2206206994597C13D831ec7",
				Amount: sdkmath.NewIntFromUint64(3),
			}},
		},
		Tags: []string{"anime", "onepiece", "series", "cartoon", "luffy", "pirate"},
	}

	chainClient, err := chainclient.NewChainClient(
		clientCtx,
		network.ChainGrpcEndpoint,
		common.OptionTLSCert(network.ChainTlsCert),
		common.OptionGasPrices("500000000lux"),
	)

	if err != nil {
		fmt.Println(err)
	}

	//AsyncBroadcastMsg, SyncBroadcastMsg, QueueBroadcastMsg
	err = chainClient.QueueBroadcastMsg(msg)
	if err != nil {
		fmt.Println(err)
	}
	chainClient.BroadcastDone()
}
