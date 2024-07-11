package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	_ "embed"

	astromeshtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"
	strategytypes "github.com/FluxNFTLabs/sdk-go/chain/modules/strategy/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/gogoproto/jsonpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	network := common.LoadNetwork("local", "")
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

	// prepare tx msg
	if err != nil {
		panic(err)
	}

	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	schema, err := os.ReadFile(dir + "/examples/chain/36_ConfigAmmSolver/schema.json")
	if err != nil {
		panic(err)
	}

	var schemaStruct strategytypes.Schema
	if err := json.Unmarshal(schema, &schemaStruct); err != nil {
		panic(err)
	}

	arbitrageQuery := schemaStruct.Groups[0].Prompts["arbitrage"].Query
	arbitrageQueryBz, _ := json.Marshal(arbitrageQuery)
	var fisQueryRequest astromeshtypes.FISQueryRequest
	if err := jsonpb.UnmarshalString(string(arbitrageQueryBz), &fisQueryRequest); err != nil {
		panic(err)
	}

	fmt.Println("sender account:", senderAddress.String())
	msg := &strategytypes.MsgTriggerStrategies{
		Sender: senderAddress.String(),
		Ids:    []string{"e9c9b5d050324513606a8f57e95c85a0b61a6941bca8ee25f85b2ef19b55ba92"},
		Inputs: [][]byte{
			[]byte(`{"arbitrage":{"pair":"btc-usdt","amount":"10000000","min_profit":"100000"}}`),
		},
		Queries: []*astromeshtypes.FISQueryRequest{&fisQueryRequest},
	}

	res, err := chainClient.SyncBroadcastMsg(msg)
	if err != nil {
		panic(err)
	}

	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
}
