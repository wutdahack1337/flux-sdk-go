package main

import (
	"fmt"
	"os"
	"strings"

	astromeshtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"
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
	if err != nil {
		panic(err)
	}

	msg := &strategytypes.MsgTriggerStrategies{
		Sender: senderAddress.String(),
		Ids:    []string{"F03BE7C54C3B01DFA36118EF104AE8C4FCD82C32CAC6E6D0D23D832AA207BDDB"},
		Inputs: [][]byte{
			[]byte(`{"withdraw_all_planes":{}}`),
		},
		Queries: []*astromeshtypes.FISQueryRequest{
			{
				Instructions: []*astromeshtypes.FISQueryInstruction{
					{
						Plane:   astromeshtypes.Plane_COSMOS,
						Action:  astromeshtypes.QueryAction_COSMOS_ASTROMESH_BALANCE,
						Address: nil,
						Input: [][]byte{
							[]byte(senderAddress.String()),
							[]byte("usdt"),
						},
					},
				},
			},
		},
	}

	//AsyncBroadcastMsg, SyncBroadcastMsg, QueueBroadcastMsg
	res, err := chainClient.SyncBroadcastMsg(msg)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(res)
}
