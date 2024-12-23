package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	astromeshtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"
	strategytypes "github.com/FluxNFTLabs/sdk-go/chain/modules/strategy/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
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
		panic(err)
	}

	fmt.Println("sender address:", senderAddress.String())
	id := "8a4bd4d7722ebebbf3e3b846574e98d4085ddbcdba95b0f97c1ad917e9ea146d"
	poolAddress := "lux1z5mjdzgkqlpqs7rr97tk4xe83ds3wtvnw4advq"
	msgTriggerStategy := &strategytypes.MsgTriggerStrategies{
		Sender: senderAddress.String(),
		Ids:    []string{id},
		Inputs: [][]byte{
			[]byte(
				`{"trade":{"action":"sell","denom":"astromesh/lux1jcltmuhplrdcwp7stlr4hlhlhgd4htqhu86cqx/CHILLGUYC","amount":"65962511937500000","slippage":"1000"}}`,
			),
		},
		Queries: []*astromeshtypes.FISQueryRequest{
			{
				Instructions: []*astromeshtypes.FISQueryInstruction{
					{
						Plane:   astromeshtypes.Plane_COSMOS,
						Action:  astromeshtypes.QueryAction_COSMOS_QUERY,
						Address: nil,
						Input: [][]byte{
							[]byte("/flux/interpool/v1beta1/pools/" + hex.EncodeToString(sdk.MustAccAddressFromBech32(poolAddress))),
						},
					},
				},
			},
		},
	}
	res, err := chainClient.SyncBroadcastMsg(msgTriggerStategy)
	if err != nil {
		panic(err)
	}

	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
}
