package main

import (
	"fmt"
	"os"
	"strings"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	astromeshtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
)

func main() {
	network := common.LoadNetwork("local", "")
	kr, err := keyring.New(
		"fluxd",
		"file",
		os.Getenv("HOME")+"/.fluxd",
		strings.NewReader("12345678\n"),
		chainclient.GetCryptoCodec(),
	)
	if err != nil {
		panic(err)
	}

	// init grpc connection
	cc, err := grpc.Dial(network.ChainGrpcEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	// init client ctx
	clientCtx, senderAddress, err := chaintypes.NewClientContext(
		network.ChainId,
		"user1",
		kr,
	)
	if err != nil {
		panic(err)
	}
	clientCtx = clientCtx.WithGRPCClient(cc)

	// init chain client
	chainClient, err := chainclient.NewChainClient(
		clientCtx,
		common.OptionGasPrices("500000000lux"),
	)
	if err != nil {
		panic(err)
	}

	// prepare tx msg
	msg1 := &astromeshtypes.MsgAstroTransfer{
		Sender:   senderAddress.String(),
		Receiver: senderAddress.String(),
		SrcPlane: astromeshtypes.Plane_COSMOS,
		DstPlane: astromeshtypes.Plane_SVM,
		Coin: sdk.Coin{
			Denom:  "btc",
			Amount: math.NewIntFromUint64(10000000000),
		},
	}
	txResp, err := chainClient.SyncBroadcastMsg(msg1)
	if err != nil {
		panic(err)
	}
	fmt.Println("resp:", txResp.TxResponse.TxHash)
	fmt.Println("gas used/want:", txResp.TxResponse.GasUsed, "/", txResp.TxResponse.GasWanted)

	msg2 := &astromeshtypes.MsgAstroTransfer{
		Sender:   senderAddress.String(),
		Receiver: senderAddress.String(),
		SrcPlane: astromeshtypes.Plane_COSMOS,
		DstPlane: astromeshtypes.Plane_SVM,
		Coin: sdk.Coin{
			Denom:  "usdt",
			Amount: math.NewIntFromUint64(10000000000),
		},
	}
	txResp, err = chainClient.SyncBroadcastMsg(msg2)
	if err != nil {
		panic(err)
	}
	fmt.Println("resp:", txResp.TxResponse.TxHash)
	fmt.Println("gas used/want:", txResp.TxResponse.GasUsed, "/", txResp.TxResponse.GasWanted)
}
