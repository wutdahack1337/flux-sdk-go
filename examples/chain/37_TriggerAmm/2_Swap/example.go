package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	_ "embed"

	"cosmossdk.io/math"
	astromeshtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"
	strategytypes "github.com/FluxNFTLabs/sdk-go/chain/modules/strategy/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/gogoproto/jsonpb"
	"github.com/mr-tron/base58"
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
	if err != nil {
		panic(err)
	}

	// check if account is linked, not then create
	isSvmLinked, svmPubkey, err := chainClient.GetSVMAccountLink(context.Background(), senderAddress)
	if err != nil {
		panic(err)
	}
	if !isSvmLinked {
		svmKey := ed25519.GenPrivKey() // Good practice: Backup this private key
		res, err := chainClient.LinkSVMAccount(svmKey, math.NewInt(1000000000000000000))
		if err != nil {
			panic(err)
		}
		fmt.Println("linked sender to svm address:", base58.Encode(svmKey.PubKey().Bytes()), "txHash:", res.TxResponse.TxHash)
	} else {
		fmt.Println("sender is already linked to svm address:", svmPubkey.String())
	}

	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	schema, err := os.ReadFile(dir + "/examples/chain/36_ConfigAmmSolver/schema.json")
	if err != nil {
		panic(err)
	}

	var schemaStruct strategytypes.Schema
	if err := json.Unmarshal(schema, &schemaStruct); err != nil {
		panic(err)
	}

	swapQuery := schemaStruct.Groups[0].Prompts["swap"].Query
	swapQueryBz, _ := json.Marshal(swapQuery)
	var fisQueryRequest astromeshtypes.FISQueryRequest
	if err := jsonpb.UnmarshalString(string(swapQueryBz), &fisQueryRequest); err != nil {
		panic(err)
	}

	// replace wallet address in schema
	fisQueryRequest.Instructions[0].Input[0] = []byte(
		strings.Replace(string(fisQueryRequest.Instructions[0].Input[0]), "${wallet}", senderAddress.String(), 1),
	)

	fmt.Println("sender account:", senderAddress.String())
	msg := &strategytypes.MsgTriggerStrategies{
		Sender: senderAddress.String(),
		Ids:    []string{"c034a7b709c7656b453e4638026b4c112a2674de88cfb8cad9a6874b931b0326"},
		Inputs: [][]byte{
			[]byte(`{"swap":{"dex_name":"evm uniswap","src_denom":"btc","dst_denom":"usdt","amount":"100"}}`),
		},
		Queries: []*astromeshtypes.FISQueryRequest{&fisQueryRequest},
	}

	res, err := chainClient.SyncBroadcastMsg(msg)
	if err != nil {
		panic(err)
	}

	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
}
