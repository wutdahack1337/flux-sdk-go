package main

import (
	sdkmath "cosmossdk.io/math"
	"fmt"
	fnfttypes "github.com/FluxNFTLabs/sdk-go/chain/modules/fnft/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"os"
	"strings"
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

	kr, err := keyring.New(
		"fluxd",
		"file",
		os.Getenv("HOME")+"/.fluxd",
		strings.NewReader("12345678"),
		chainclient.GetCryptoCodec(),
	)
	if err != nil {
		panic(err)
	}

	// initialize grpc client
	clientCtx, senderAddress, err := chaintypes.NewClientContext(
		network.ChainId,
		"user1",
		kr,
	)
	if err != nil {
		panic(err)
	}
	clientCtx = clientCtx.WithNodeURI(network.TmEndpoint).WithClient(tmClient)

	// prepare tx msg
	msg := &fnfttypes.MsgCreate{
		Sender:  senderAddress.String(),
		ClassId: "series",
		Supply:  sdkmath.NewIntFromUint64(7000),
		InitialPrice: sdktypes.Coin{
			Denom:  "ibc0xdAC17F958D2ee523a2206206994597C13D831ec7",
			Amount: sdkmath.NewIntFromUint64(1500000),
		},
		ISOTimestamp:         uint64(time.Now().Unix() + 25),
		ISOSuccessPercent:    75,
		AcceptedPaymentDenom: "ibc0xdAC17F958D2ee523a2206206994597C13D831ec7",
		DividendInterval:     864000,
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
