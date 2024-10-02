package main

import (
	"fmt"
	"os"
	"strings"

	"cosmossdk.io/math"

	"github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"
	strategytypes "github.com/FluxNFTLabs/sdk-go/chain/modules/strategy/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	network := common.LoadNetwork("local", "")
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

	// prepare tx msg
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	bz, err := os.ReadFile(dir + "/examples/chain/26_MsgConfigCron/cron.wasm")
	if err != nil {
		panic(err)
	}
	fmt.Println("sender: ", senderAddress.String())
	msg := &strategytypes.MsgConfigStrategy{
		Sender:   senderAddress.String(),
		Config:   strategytypes.Config_update,
		Id:       "010A79B0D5B00E9D71EB829837B3E7BFD281FE5AD6E7EAB2837D3384724D12D7",
		Strategy: bz,
		Query: &types.FISQueryRequest{
			Instructions: []*types.FISQueryInstruction{},
		},
		TriggerPermission: &strategytypes.PermissionConfig{
			Type:      strategytypes.AccessType_only_addresses,
			Addresses: []string{senderAddress.String()},
		},
		Metadata: &strategytypes.StrategyMetadata{
			Name:         "bank cron v1.2",
			Description:  "transfer _ usdt to account _ every block",
			Logo:         "",
			Website:      "",
			Type:         strategytypes.StrategyType_CRON,
			Tags:         []string{"cron", "bank", "util", "updated"},
			Schema:       "",
			CronGasPrice: math.NewIntFromUint64(600000000),
			CronInput: `{
			  "receiver": "lux158ucxjzr6ccrlpmz8z05wylu8tr5eueqcp2afu",
			  "amount": "5",
			  "denom": "lux"
			}`,
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
