package main

import (
	sdkmath "cosmossdk.io/math"
	"fmt"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"os"
)

func main() {
	tmClient, err := rpchttp.New("http://localhost:26657", "/websocket")
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
		"flux-1",
		senderAddress.String(),
		cosmosKeyring,
	)
	if err != nil {
		fmt.Println(err)
	}
	clientCtx = clientCtx.WithClient(tmClient)

	// prepare tx msg
	msg := &banktypes.MsgSend{
		FromAddress: senderAddress.String(),
		ToAddress:   "lux1jcltmuhplrdcwp7stlr4hlhlhgd4htqhu86cqx",
		Amount: []sdktypes.Coin{{
			Denom: "lux", Amount: sdkmath.NewInt(1000000000000000000)}, // 1 LUX
		},
	}

	chainClient, err := chainclient.NewChainClient(
		clientCtx,
		"localhost:9900",
		//common.OptionTLSCert(network.ChainTlsCert),
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
