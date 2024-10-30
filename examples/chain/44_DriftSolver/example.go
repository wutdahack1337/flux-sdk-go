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
			Description: "Solver simplifies drift interactions. Available options:  leverage: 1..20, market: 'btc-usdt', 'eth-usdt', 'sol-usdt', auction_duration: 10..255, percent: 1..100, taker_svm_address: base58 pubkey of taker",
			Logo:        "https://img.icons8.com/?size=100&id=Wnx66N0cnKa7&format=png&color=000000",
			Website:     "https://www.astromesh.xyz",
			Type:        strategytypes.StrategyType_INTENT_SOLVER,
			Tags:        strings.Split("Solver, Bank, Utility", ", "),
			Schema:      `{
				"groups": [
				  {
					"name": "",
					"prompts": {
					  "place_perp_market_order": {
						"template": "open a ${direction:string} position, margin ${usdt_amount:number} usdt, ${leverage:number}x leverage on ${market:string} market, with ${auction_duration:number} blocks auction time",
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
						"template": "fill ${percent:number}% of order ${taker_order_id:number} from ${taker_svm_address:string}",
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
								"e3twZGEgInVzZXIiIChkZWNvZGVCYXNlNTggdGFrZXJfc3ZtX2FkZHJlc3MpICIAACIgIkZMUjNtZllyTVpVbmhxRWFkTkpWd2pVaGpYOGt5OXZFOXFUdERta0s0dndDIn19"
							  ]
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