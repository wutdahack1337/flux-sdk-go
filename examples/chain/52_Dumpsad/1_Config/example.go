package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	_ "embed"

	"cosmossdk.io/math"
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
	//go:embed dumpsad_solver.wasm
	intentSolverBinary []byte

	//go:embed dumpsad_cron.wasm
	cronBinary []byte
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
			Name:        "Dumpsad meme",
			Description: "Hehe dump dump dump",
			Logo:        "https://img.icons8.com/?size=100&id=Wnx66N0cnKa7&format=png&color=000000",
			Website:     "https://www.astromesh.xyz",
			Type:        strategytypes.StrategyType_INTENT_SOLVER,
			Tags:        strings.Split("Solver, Bank, Utility", ", "),
			Schema:      `{}`,
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
	solverId := response.Id
	fmt.Println("dumpsad solver id:", solverId)

	// config cron
	msg = &strategytypes.MsgConfigStrategy{
		Sender:   senderAddress.String(),
		Config:   strategytypes.Config_deploy,
		Id:       "",
		Strategy: cronBinary,
		Query: &types.FISQueryRequest{
			Instructions: []*types.FISQueryInstruction{
				{
					Plane:  types.Plane_COSMOS,
					Action: types.QueryAction_COSMOS_EVENT,
					Input: [][]byte{
						[]byte("strategy,flux.strategy.v1beta1.StrategyEvent"),
					},
				},
				{
					Plane:  types.Plane_COSMOS,
					Action: types.QueryAction_COSMOS_KVSTORE,
					Input: [][]byte{
						// 	"wasm".as_bytes().to_vec(),
						// [&[4u8], "lastContractId".as_bytes()].concat().to_vec(),
						[]byte("wasm"),
						append([]byte{4}, []byte("lastContractId")...),
					},
				},
			},
		},
		Metadata: &strategytypes.StrategyMetadata{
			Name:         "Dumpsad graduate",
			Description:  "Just graduate",
			Logo:         "https://img.icons8.com/?size=100&id=Wnx66N0cnKa7&format=png&color=000000",
			Website:      "https://www.astromesh.xyz",
			Type:         strategytypes.StrategyType_CRON,
			Tags:         strings.Split("Solver, Bank, Utility", ", "),
			Schema:       `{}`,
			CronInput:    fmt.Sprintf(`{"solver_id":"%s"}`, solverId),
			CronInterval: 0, // event listener
			CronGasPrice: math.NewInt(500_000_000),
		},
	}

	res, err = chainClient.SyncBroadcastMsg(msg)
	if err != nil {
		panic(err)
	}

	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)

	hexResp, err = hex.DecodeString(res.TxResponse.Data)
	if err != nil {
		panic(err)
	}

	var txData2 sdk.TxMsgData
	if err := txData2.Unmarshal(hexResp); err != nil {
		panic(err)
	}

	var response2 strategytypes.MsgConfigStrategyResponse
	if err := response2.Unmarshal(txData2.MsgResponses[0].Value); err != nil {
		panic(err)
	}
	fmt.Println("dumpsad cron id:", response2.Id)
}
