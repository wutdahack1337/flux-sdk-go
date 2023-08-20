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
		os.Getenv("HOME")+"/.luxd",
		"luxd",
		"file",
		"lux-user",
		"12345678",
		"88CBEAD91AEE890D27BF06E003ADE3D4E952427E88F88D31D61D3EF5E5D54305", // keyring will be used if pk not provided
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
	msg := &fnfttypes.MsgCreate{
		Sender:  senderAddress.String(),
		ClassId: "series",
		Uri:     "https://cocacola.com/ads/flux/0",
		Supply:  sdkmath.NewIntFromUint64(7000),
		InitialPrice: sdktypes.Coin{
			Denom:  "ibc0xdAC17F958D2ee523a2206206994597C13D831ec7",
			Amount: sdkmath.NewIntFromUint64(1500000),
		},
		ISOTimestamp:             uint64(time.Now().Unix() + 864000),
		AcceptedSponsorshipDenom: "ibc0xdAC17F958D2ee523a2206206994597C13D831ec7",
		DividendInterval:         864000,
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
