package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"cosmossdk.io/math"

	_ "embed"

	"github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"
	interpooltypes "github.com/FluxNFTLabs/sdk-go/chain/modules/interpool/types"
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

	cronBinary, err := os.ReadFile("/Users/phucta/flux/nexus-bots/examples/cron/pool-cron/target/wasm32-unknown-unknown/release/pool_cron.wasm")
	if err != nil {
		panic(err)
	}

	poolId := "600edb5c594e1f0fe0791f7f2f8501ff9dae917491ea3b683ef10814f4b87870"
	msg := &strategytypes.MsgConfigStrategy{
		Sender:   senderAddress.String(),
		Config:   strategytypes.Config_deploy,
		Id:       "",
		Strategy: cronBinary,
		Query: &types.FISQueryRequest{
			// track balances of accounts you want to make the amount even
			Instructions: []*types.FISQueryInstruction{
				{
					Plane:   types.Plane_COSMOS,
					Action:  types.QueryAction_COSMOS_QUERY,
					Address: []byte{},
					Input: [][]byte{
						[]byte("/flux/interpool/v1beta1/pools/" + poolId),
					},
				},
			},
		},
		TriggerPermission: &strategytypes.PermissionConfig{
			Type:      strategytypes.AccessType_only_addresses,
			Addresses: []string{senderAddress.String()},
		},
		Metadata: &strategytypes.StrategyMetadata{
			Name:         "Bank Cron Demo",
			Description:  "Transfer _ usdt to account _ at _ interval",
			Logo:         "https://img.icons8.com/?size=100&id=eXagLnlG4m29&format=png&color=000000",
			Website:      "https://www.astromesh.xyz/",
			Type:         strategytypes.StrategyType_CRON,
			Tags:         []string{"cron", "bank", "pool"},
			Schema:       "",
			CronGasPrice: math.NewIntFromUint64(500000000),
			CronInput:    `{}`,
			CronInterval: 2,
		},
	}

	//AsyncBroadcastMsg, SyncBroadcastMsg, QueueBroadcastMsg
	res, err := chainClient.SyncBroadcastMsg(msg)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("deployed cron: tx hash", res.TxResponse.TxHash)
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
	msgUpdatePool := &interpooltypes.MsgUpdatePool{
		Sender: senderAddress.String(),
		PoolId: poolId,
		CronId: response.Id,
	}

	res, err = chainClient.SyncBroadcastMsg(msgUpdatePool)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("updated pool. tx hash", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
}
