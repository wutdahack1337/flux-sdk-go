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
	//go:embed amm_solver.wasm
	strategyBinary []byte

	//go:embed schema.json
	schema []byte
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
			Name:        "AMM Solver",
			Description: "Versatile solver designed to simplify swap and arbitrage operations across all Automated Market Makers (AMMs) in all Planes including Uniswap on EVM, Astroport on WasmVM and Raydium on SVM.\n\ndex_name options: wasm astroport, evm uniswap, svm raydium\npair options: btc-usdt, eth-usdt, sol-usdt",
			Logo:        "https://img.icons8.com/?size=100&id=DRqSfJR0cb56&format=png&color=000000",
			Website:     "https://www.astromesh.xyz/",
			Type:        strategytypes.StrategyType_INTENT_SOLVER,
			Tags:        strings.Split("Solver, Uniswap, Astroport, Raydium, DeFi, Arbitrage", ", "),
			Schema:      string(schema),
			SupportedApps: []*strategytypes.SupportedApp{
				{
					Name:            "Uniswap",
					ContractAddress: "e2f81b30e1d47dffdbb6ab41ec5f0572705b026d",
					Plane:           types.Plane_EVM,
					Verified:        false,
				},
				{
					Name:            "Astroport",
					ContractAddress: "lux14hj2tavq8fpesdwxxcu44rty3hh90vhujrvcmstl4zr3txmfvw9sm3tpfk",
					Plane:           types.Plane_WASM,
					Verified:        false,
				},
				{
					Name:            "Drift",
					ContractAddress: "FLR3mfYrMZUnhqEadNJVwjUhjX8ky9vE9qTtDmkK4vwC",
					Plane:           types.Plane_SVM,
					Verified:        false,
				},
			},
		},
	}

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

	fmt.Println("deployed, intent solver id:", response.Id)
	fmt.Println("hint: use this Id to trigger it in examples/chain/37_TriggerAmmArbitrage/example.go !!!")
}
