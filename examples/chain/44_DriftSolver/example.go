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

var (
	//go:embed drift_solver.wasm
	intentSolverBinary []byte
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
		fmt.Println(err)
	}

	msg := &strategytypes.MsgConfigStrategy{
		Sender:   senderAddress.String(),
		Config:   strategytypes.Config_deploy,
		Id:       "",
		Strategy: intentSolverBinary,
		Query:    &types.FISQueryRequest{},
		Metadata: &strategytypes.StrategyMetadata{
			Name:        "Nexus Transfer Solver",
			Description: "Simplifies financial transfers by allowing retail users to batch multiple requests using easy, human-readable prompts.",
			Logo:        "https://img.icons8.com/?size=100&id=Wnx66N0cnKa7&format=png&color=000000",
			Website:     "https://www.astromesh.xyz",
			Type:        strategytypes.StrategyType_INTENT_SOLVER,
			Tags:        strings.Split("Solver, Bank, Utility", ", "),
			Schema:      `
				{
					"groups": [
					  {
						"name": "Nexus Transfer Solver",
						"prompts": {
						  "place_perp_market_order": {
							"template": "open position ${usdt_amount:number} USDT, ${leverage:number} leverage on ${market:string} market, with ${auction_duration:number} blocks auction time",
							"msg_fields": [
								"usdt_amount",
								"leverage",
								"market",
							 	"auction_duration"
							],
							"query": {
							  "instructions": [
								{
								  "plane": "COSMOS",
								  "action": "COSMOS_QUERY",
								  "address": "",
								  "input": []
								}
							  ]
							}
						  },
						  "fill_perp_market_order": {
							"template": "fill ${percent:number} of order ${taker_order_id:number} from ${taker_svm_address:string}",
							"msg_fields": [
							  "taker_svm_address",
							  "taker_order_id",
							  "percent"
							],
							"query": {
							  "instructions": [
								{
								  "plane": "COSMOS",
								  "action": "COSMOS_QUERY",
								  "address": "",
								  "input": []
								}
							  ]
							}
						  }
						}
					  }
					]
				  }`,
		},
	}

	//AsyncBroadcastMsg, SyncBroadcastMsg, QueueBroadcastMsg
	res, err := chainClient.SyncBroadcastMsg(msg)
	if err != nil {
		fmt.Println(err)
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