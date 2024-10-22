package main

import (
	"fmt"
	"os"
	"strings"

	"cosmossdk.io/math"

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
	//go:embed compound_staking_cron.wasm
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

	fmt.Println("sender: ", senderAddress.String())

	url := fmt.Sprintf(
		"/cosmos/distribution/v1beta1/delegators/%s/rewards", 
		senderAddress.String(),
	)
	
	msg := &strategytypes.MsgConfigStrategy{
		Sender:   senderAddress.String(),
		Config:   strategytypes.Config_deploy,
		Id:       "",
		Strategy: cronBinary,
		Query: &types.FISQueryRequest{
			// track balances of accounts you want to make the amount even
			Instructions: []*types.FISQueryInstruction{
				{
					Plane: types.Plane_COSMOS,
					Action: types.QueryAction_COSMOS_QUERY,
					Address: []byte{},
					Input: [][]byte{
						[]byte(url),
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
			Tags:         []string{"cron", "bank", "util"},
			Schema:       "",
			CronGasPrice: math.NewIntFromUint64(500000000),
			CronInput:    `{}`,
			CronInterval: 5,
		},
	}

	//AsyncBroadcastMsg, SyncBroadcastMsg, QueueBroadcastMsg
	res, err := chainClient.SyncBroadcastMsg(msg)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
}
