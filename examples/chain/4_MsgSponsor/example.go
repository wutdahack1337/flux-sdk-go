package main

import (
	sdkmath "cosmossdk.io/math"
	"fmt"
	fnfttypes "github.com/FluxNFTLabs/sdk-go/chain/modules/fnft/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	"os"
	"time"

	"github.com/FluxNFTLabs/sdk-go/client/common"

	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
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
		"user3",
		"12345678",
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
	msg := &fnfttypes.MsgSponsor{
		Sender:      senderAddress.String(),
		ClassId:     "series",
		Id:          "0",
		Coin:        sdktypes.Coin{Denom: "ibc0xdAC17F958D2ee523a2206206994597C13D831ec7", Amount: sdkmath.NewIntFromUint64(1500000)},
		Description: "Cocacola Ad",
		Uri:         "https://flux.com/ads/cocacola/1",
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

	time.Sleep(time.Second * 5)

	gasFee, err := chainClient.GetGasFee()
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("gas fee:", gasFee, "LUX")
}
