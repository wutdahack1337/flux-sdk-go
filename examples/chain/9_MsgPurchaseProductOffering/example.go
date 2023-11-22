package main

import (
	"fmt"
	bazaartypes "github.com/FluxNFTLabs/sdk-go/chain/modules/bazaar/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"os"
	"strings"

	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
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
	msg := &bazaartypes.MsgPurchaseOffering{
		Sender:           senderAddress.String(),
		ClassId:          "series",
		Id:               "0",
		ProductId:        "0",
		OfferingIdx:      []uint64{0},
		OfferingQuantity: []uint64{7},
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
