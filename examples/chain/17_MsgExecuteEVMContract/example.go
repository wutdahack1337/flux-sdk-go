package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	evmtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/evm/types"
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

	// client should know contract's ABI to build this payload
	callData, _ := hex.DecodeString("e942b5160000000000000000000000000000000000000000000000000000000000000040000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000056f776e657200000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000075068756320546100000000000000000000000000000000000000000000000000")
	contractAddress, _ := hex.DecodeString("07aa076883658b7ed99d25b1e6685808372c8fe2")

	// prepare tx msg
	msg := &evmtypes.MsgExecuteContract{
		Sender:          senderAddress.String(),
		ContractAddress: contractAddress,
		Calldata:        callData,
	}

	txResp, err := chainClient.SyncBroadcastMsg(msg)
	if err != nil {
		panic(err)
	}
	fmt.Println("resp:", txResp.TxResponse.TxHash)
	fmt.Println("gas used/want:", txResp.TxResponse.GasUsed, "/", txResp.TxResponse.GasWanted)

	// to double check locally:
	// http://localhost:10337/flux/evm/v1beta1/query/{address}/{calldata}
	// http://localhost:10337/flux/evm/v1beta1/query/a7f16731951d943768cf2053485b69ef61fef8be/aT7IXgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAVvd25lcgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==
}
