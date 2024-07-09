package main

import (
	"fmt"
	"os"
	"strings"

	"cosmossdk.io/math"

	_ "embed"

	"github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"
	astromeshtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"
	strategytypes "github.com/FluxNFTLabs/sdk-go/chain/modules/strategy/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

//go:embed evm_deploy_watcher.wasm
var strategyBinary []byte

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
		panic(err)
	}

	// prepare tx msg
	fmt.Println("sender:", senderAddress.String())
	msg := &strategytypes.MsgConfigStrategy{
		Sender:   senderAddress.String(),
		Config:   strategytypes.Config_deploy,
		Strategy: strategyBinary,
		Query: &types.FISQueryRequest{
			// track balances of accounts you want to make the amount even
			Instructions: []*types.FISQueryInstruction{
				{
					Plane:   astromeshtypes.Plane_COSMOS,
					Action:  astromeshtypes.QueryAction_COSMOS_EVENT,
					Address: nil,
					Input: [][]byte{
						[]byte("evm,EvmDeployContract,flux.evm.v1beta1.EventDeploy"),
					},
				},
			},
		},
		TriggerPermission: &strategytypes.PermissionConfig{
			Type:      strategytypes.AccessType_only_addresses,
			Addresses: []string{senderAddress.String()},
		},
		Metadata: &strategytypes.StrategyMetadata{
			Name:         "evm deploy event watcher",
			Description:  "watch evm deploy",
			Logo:         "",
			Website:      "",
			Type:         strategytypes.StrategyType_CRON,
			Tags:         []string{"cron", "bank", "util"},
			Schema:       "",
			CronGasPrice: math.NewIntFromUint64(500000000),
			CronInput: `{
			  "amount": "1",
			  "denom": "usdt"
			}`,
			CronInterval: 0,
		},
	}

	//AsyncBroadcastMsg, SyncBroadcastMsg, QueueBroadcastMsg
	res, err := chainClient.SyncBroadcastMsg(msg)
	if err != nil {
		panic(err)
	}

	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
	fmt.Println("now please run evm deploy contract at examples/chain/16_MsgDeployEVMContract and contract owner's usdt amount automatically increased by 1")
}
