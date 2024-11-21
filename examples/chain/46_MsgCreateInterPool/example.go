package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	interpooltypes "github.com/FluxNFTLabs/sdk-go/chain/modules/interpool/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	networkName := "local"
	if len(os.Args) > 1 {
		networkName = os.Args[1]
	}
	network := common.LoadNetwork(networkName, "")
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

	// init grpc connection
	cc, err := grpc.Dial(network.ChainGrpcEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	// init client ctx
	clientCtx, senderAddress, err := chaintypes.NewClientContext(
		network.ChainId,
		"user2",
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
		fmt.Println(err)
	}

	fmt.Println("sender address:", senderAddress.String())
	msg := &interpooltypes.MsgCreatePool{
		Sender: senderAddress.String(),
		OperatorCommissionConfig: &interpooltypes.CommissionConfig{
			ManagementFeeRate:     100,
			ManagementFeeInterval: 2,
			TradingFeeRate:        100,
		},
	}
	res, err := chainClient.SyncBroadcastMsg(msg)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("tx hash", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)

	hexResp, err := hex.DecodeString(res.TxResponse.Data)
	if err != nil {
		panic(err)
	}

	var txData sdk.TxMsgData
	if err := txData.Unmarshal(hexResp); err != nil {
		panic(err)
	}

	var response interpooltypes.MsgCreatePoolResponse
	if err := response.Unmarshal(txData.MsgResponses[0].Value); err != nil {
		panic(err)
	}

	fmt.Println("pool id:", response.PoolId)
}
