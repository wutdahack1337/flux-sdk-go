package main

import (
	"fmt"
	"os"
	"strings"

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
	cc, err := grpc.Dial("localhost:9900", grpc.WithTransportCredentials(insecure.NewCredentials()))
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

	bz, err := os.ReadFile(dir + "/examples/chain/24_ConfigIntentSolver/plane_util.wasm")
	if err != nil {
		panic(err)
	}

	msg := &strategytypes.MsgConfigStrategy{
		Sender:   senderAddress.String(),
		Config:   strategytypes.Config_deploy,
		Id:       "",
		Strategy: bz,
		Query:    &types.FISQueryRequest{},
		Metadata: &strategytypes.StrategyMetadata{
			Name:        "Astromesh plane util",
			Description: "Astromesh transfer intent solver that helps user manage tokens easily",
			Logo:        "https://media.licdn.com/dms/image/D560BAQG83Lxt5OyW4Q/company-logo_200_200/0/1706516333778?e=1724889600&v=beta&t=_fD4VtBEpNKJ8gESgFOcZyTDrQFlRUfxw1iQD8mItnM",
			Website:     "https://www.astromesh.xyz",
			Type:        strategytypes.StrategyType_INTENT_SOLVER,
			Tags:        []string{"helper"},
			Schema:      `{"groups":[{"name":"transfer helper","prompts":{"withdraw_all_planes":{"template":"withdraw ${denom:string} from planes to cosmos","query":{"instructions":[{"plane":"COSMOS","action":"COSMOS_ASTROMESH_BALANCE","address":"","input":["JHt3YWxsZXR9","JHtkZW5vbX0="]}]}}}}]}`,
		},
	}

	//AsyncBroadcastMsg, SyncBroadcastMsg, QueueBroadcastMsg
	res, err := chainClient.SyncBroadcastMsg(msg)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
}
