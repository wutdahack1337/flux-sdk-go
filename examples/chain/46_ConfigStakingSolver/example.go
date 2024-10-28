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
			Description: "Simplifies financial transfers by allowing retail users to batch multiple requests using easy, human-readable prompts.",
			Logo:        "https://img.icons8.com/?size=100&id=Wnx66N0cnKa7&format=png&color=000000",
			Website:     "https://www.astromesh.xyz",
			Type:        strategytypes.StrategyType_INTENT_SOLVER,
			Tags:        strings.Split("Solver, Bank, Utility", ", "),
			Schema:      `
			{
				"groups": [
				  {
					"name": "Staking Solver",
					"prompts": {
					  "stake_default": {
						"template": "stake ${amount:number} lux",
						"msg_fields": [
						  "amount"
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
					  "stake": {
						"template": "stake ${amount:number} lux with validator at ${validator_address:string}",
						"msg_fields": [
						  "amount",
						  "validator_address"
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
					  "re_delegate": {
						"template": "move ${amount:number} lux from validator ${src_validator_address:string} to ${new_validator_address:string}",
						"msg_fields": [
						  "amount",
						  "src_validator_address",
						  "new_validator_address"
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
					  "claim_all_rewards": {
						"template": "collect all accumulated staking rewards",
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
					  "unstake_all": {
						"template": "withdraw all staked lux",
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
					  "claim_rewards_and_restake": {
						"template": "claim rewards and restake",
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
