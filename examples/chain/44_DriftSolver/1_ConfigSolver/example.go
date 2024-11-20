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
		panic(err)
	}

	msg := &strategytypes.MsgConfigStrategy{
		Sender:   senderAddress.String(),
		Config:   strategytypes.Config_deploy,
		Id:       "",
		Strategy: intentSolverBinary,
		Query:    &types.FISQueryRequest{},
		Metadata: &strategytypes.StrategyMetadata{
			Name:        "Drift Solver",
			Description: "The Drift solver for Drift Protocol v2 simplifies drift order creation and matching, enhancing liquidity management and minimizing slippage for improved trade outcomes\n\n**Available options:**\n\ndirection: `long`, `short`\n\nleverage: `1..20`, \n\nmarket: `btc-usdt`, `eth-usdt`, `sol-usdt`, \n\nauction_duration: `10..255`, \n\npercent: `1..100`,\n\ntaker_svm_address: base58 pubkey of taker\n",
			Logo:        "https://camo.githubusercontent.com/1359e372dc137431980bd1899fa66aa67bb317c8af840decedbcc74d7434c160/68747470733a2f2f75706c6f6164732d73736c2e776562666c6f772e636f6d2f3631313538303033356164353962323034333765623032342f3631366639376134326635363337633435313764303139335f4c6f676f2532302831292532302831292e706e67",
			Website:     "https://www.drift.trade",
			Type:        strategytypes.StrategyType_INTENT_SOLVER,
			Tags:        strings.Split("Drift, DeFi", ", "),
			Schema: `{
				"groups": [
					{
					"name": "",
					"prompts": {
						"place_perp_market_order": {
						"template": "open ${direction:string} market take order, margin ${usdt_amount:number} usdt, ${leverage:number}x leverage on ${market:string} market, with ${auction_duration:number} blocks auction time",
						"msg_fields": [
							"direction",
							"market",
							"usdt_amount",
							"leverage",
							"auction_duration"
						],
						"query": {
							"instructions": [
							{
								"plane": "COSMOS",
								"action": "COSMOS_QUERY",
								"address": "",
								"input": [
								"L2ZsdXgvc3ZtL3YxYmV0YTEvYWNjb3VudF9saW5rL2Nvc21vcy8ke3dhbGxldH0="
								]
							},
							{
								"plane": "SVM",
								"action": "VM_QUERY",
								"address": "",
								"input": [
								"e3twZGEgInVzZXIiIChkZWNvZGVCYXNlNTggc3ZtQWRkcmVzcykgIgAAIiAiRkxSM21mWXJNWlVuaHFFYWROSlZ3alVoalg4a3k5dkU5cVR0RG1rSzR2d0MifX0=",
								"YMwDCsPHRPr0xHohVBxQl+FYRFUF36nbnAN1pWrwMMw=",
								"wfqTmNCHrLG9FeC5DUYyhIr4UcF7a6KMXwRj5Flc7mo=",
								"EshKWsw7y2eqtJaEnz8s3hsFx3x7TnpgirCMBasb/X0="
								]
							}
							]
						}
						},
						"fill_perp_market_order": {
						"template": "fill JIT ${direction:string} orders of market ${market:string} at price ${price:string}, quantity ${quantity:number}",
						"msg_fields": [
							"direction",
							"market",
							"price",
							"quantity"
						],
						"query": {
							"instructions": []
						}
						}
					}
					}
				]
			}`,
			SupportedApps: []*strategytypes.SupportedApp{
				{
					Name:            "Drift protocol v2",
					ContractAddress: "FLR3mfYrMZUnhqEadNJVwjUhjX8ky9vE9qTtDmkK4vwC",
					Plane:           types.Plane_SVM,
					Verified:        false,
				},
			},
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
