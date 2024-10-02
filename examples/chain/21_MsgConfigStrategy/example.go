package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"

	_ "embed"

	"github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"
	strategytypes "github.com/FluxNFTLabs/sdk-go/chain/modules/strategy/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	//go:embed strategy.wasm
	strategyBinary []byte
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

	fmt.Println("sender: ", senderAddress.String())
	msg := &strategytypes.MsgConfigStrategy{
		Sender:   senderAddress.String(),
		Config:   strategytypes.Config_deploy,
		Id:       "",
		Strategy: strategyBinary,
		Query: &types.FISQueryRequest{
			// track balances of accounts you want to make the amount even
			Instructions: []*types.FISQueryInstruction{
				{
					Plane:   types.Plane_COSMOS,
					Action:  types.QueryAction_COSMOS_BANK_BALANCE,
					Address: []byte{},
					Input: [][]byte{
						[]byte("lux1cml96vmptgw99syqrrz8az79xer2pcgp209sv4,lux1jcltmuhplrdcwp7stlr4hlhlhgd4htqhu86cqx"),
						[]byte("lux,lux"),
					},
				},
			},
		},
		TriggerPermission: &strategytypes.PermissionConfig{
			Type:      strategytypes.AccessType_only_addresses,
			Addresses: []string{senderAddress.String()},
		},
		Metadata: &strategytypes.StrategyMetadata{
			Description: "Listen to balances change and even out account balance for certain denom",
			Type:        strategytypes.StrategyType_STRATEGY,
			Logo:        "https://img.icons8.com/?size=100&id=GRjuzy9lKwQD&format=png&color=000000",
			Website:     "https://www.astromesh.xyz/",
			SupportedApps: []*strategytypes.SupportedApp{
				{
					Name:            "Random App",
					ContractAddress: "ab6b4d064c968eca87f775d2493a222987052bc0",
					Plane:           types.Plane_EVM,
					Verified:        false,
				},
			},
		},
	}

	//AsyncBroadcastMsg, SyncBroadcastMsg, QueueBroadcastMsg
	res, err := chainClient.SyncBroadcastMsg(msg)
	if err != nil {
		fmt.Println(err)
	}

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
