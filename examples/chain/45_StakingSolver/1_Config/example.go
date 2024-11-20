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
	//go:embed staking_solver.wasm
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
			Name:        "Staking Solver",
			Description: "The staking solver on Astromesh streamlines the staking process, helping users delegate tokens securely and optimize rewards\n\n**Available options:**\n\nvalidator_name: `flux`",
			Logo:        "https://icons.veryicon.com/png/o/business/work-circle/proof-of-stake.png",
			Website:     "https://www.astromesh.xyz",
			Type:        strategytypes.StrategyType_INTENT_SOLVER,
			Tags:        strings.Split("Solver, Bank, Utility", ", "),
			Schema: `{
				"groups": [
				  {
					"name": "Staking Solver",
					"prompts": {
					  "delegate": {
						"template": "delegate ${amount:number} lux to validator ${validator_name:string}",
						"msg_fields": [
						  "amount",
						  "validator_name"
						],
						"query": {
						  "instructions": [
							{
							  "plane": "COSMOS",
							  "action": "COSMOS_QUERY",
							  "address": "",
							  "input": [
								"L2Nvc21vcy9zdGFraW5nL3YxYmV0YTEvdmFsaWRhdG9ycw=="
							  ]
							}
						  ]
						}
					  },
					  "undelegate": {
						"template": "undelegate ${amount:number} lux from validator ${validator_name:string}",
						"msg_fields": [
						  "amount",
						  "validator_name"
						],
						"query": {
						  "instructions": [
							{
							  "plane": "COSMOS",
							  "action": "COSMOS_QUERY",
							  "address": "",
							  "input": [
								"L2Nvc21vcy9kaXN0cmlidXRpb24vdjFiZXRhMS9kZWxlZ2F0b3JzLyR7d2FsbGV0fS9yZXdhcmRz"
							  ]
							},
							{
							  "plane": "COSMOS",
							  "action": "COSMOS_QUERY",
							  "address": "",
							  "input": [
								"L2Nvc21vcy9zdGFraW5nL3YxYmV0YTEvdmFsaWRhdG9ycw=="
							  ]
							}
						  ]
						}
					  },
					  "claim_all_rewards": {
						"template": "claim all rewards from all validators",
						"query": {
						  "instructions": [
							{
							  "plane": "COSMOS",
							  "action": "COSMOS_QUERY",
							  "address": "",
							  "input": [
								"L2Nvc21vcy9kaXN0cmlidXRpb24vdjFiZXRhMS9kZWxlZ2F0b3JzLyR7d2FsbGV0fS9yZXdhcmRz"
							  ]
							}
						  ]
						}
					  },
					  "claim_rewards_and_redelegate": {
						"template": "claim all rewards and delegate to same validators",
						"query": {
						  "instructions": [
							{
							  "plane": "COSMOS",
							  "action": "COSMOS_QUERY",
							  "address": "",
							  "input": [
								"L2Nvc21vcy9kaXN0cmlidXRpb24vdjFiZXRhMS9kZWxlZ2F0b3JzLyR7d2FsbGV0fS9yZXdhcmRz"
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
