package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	_ "embed"

	"github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"
	strategytypes "github.com/FluxNFTLabs/sdk-go/chain/modules/strategy/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

//go:embed amm_solver.wasm
var strategyBinary []byte

//go:embed schema.json
var schema []byte

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
	fmt.Println("intent owner:", senderAddress.String())
	msg := &strategytypes.MsgConfigStrategy{
		Sender:   senderAddress.String(),
		Config:   strategytypes.Config_deploy,
		Strategy: strategyBinary,
		Query:    &types.FISQueryRequest{},
		TriggerPermission: &strategytypes.PermissionConfig{
			Type: strategytypes.AccessType_anyone,
		},
		Metadata: &strategytypes.StrategyMetadata{
			Name:        "AMM solver wizard",
			Description: "",
			Logo:        "https://cdn-icons-png.freepik.com/512/8482/8482621.png",
			Website:     "",
			Type:        strategytypes.StrategyType_INTENT_SOLVER,
			Tags:        []string{"util", "defi"},
			Schema:      string(schema),
		},
	}

	//AsyncBroadcastMsg, SyncBroadcastMsg, QueueBroadcastMsg
	res, err := chainClient.SyncBroadcastMsg(msg)
	if err != nil {
		panic(err)
	}

	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)

	hexResp, err := hex.DecodeString(res.TxResponse.Data)
	if err != nil {
		panic(err)
	}

	// decode result to get contract address
	var txData sdk.TxMsgData
	if err := txData.Unmarshal(hexResp); err != nil {
		panic(err)
	}

	var response strategytypes.MsgConfigStrategyResponse
	if err := response.Unmarshal(txData.MsgResponses[0].Value); err != nil {
		panic(err)
	}

	fmt.Println("strategy id:", response.Id)
}
