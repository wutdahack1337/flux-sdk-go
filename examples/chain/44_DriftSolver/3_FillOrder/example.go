package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"cosmossdk.io/math"
	astromeshtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"
	strategytypes "github.com/FluxNFTLabs/sdk-go/chain/modules/strategy/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/cosmos/btcutil/base58"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/gagliardetto/solana-go"
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
		"signer4",
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
	driftProgramId := solana.MustPublicKeyFromBase58("FLR3mfYrMZUnhqEadNJVwjUhjX8ky9vE9qTtDmkK4vwC")
	// check if account is linked, not then create
	isSvmLinked, svmPubkey, err := chainClient.GetSVMAccountLink(context.Background(), senderAddress)
	if err != nil {
		panic(err)
	}

	if !isSvmLinked {
		svmKey := ed25519.GenPrivKey() // Good practice: Backup this private key
		res, err := chainClient.LinkSVMAccount(svmKey, math.NewIntFromUint64(1000_000_000_000))
		if err != nil {
			panic(err)
		}
		fmt.Println("linked sender to svm address:", base58.Encode(svmKey.PubKey().Bytes()), "txHash:", res.TxResponse.TxHash)
	} else {
		fmt.Println("sender is already linked to svm address:", svmPubkey.String())
	}

	takerKr, err := kr.Key("user1")
	if err != nil {
		panic(err)
	}

	takerAddress, err := takerKr.GetAddress()
	if err != nil {
		panic(err)
	}

	isSvmLinked, takerPubkey, err := chainClient.GetSVMAccountLink(context.Background(), takerAddress)
	if err != nil {
		panic(err)
	}

	if !isSvmLinked {
		panic(fmt.Errorf("taker is not linked: %s", takerAddress.String()))
	}

	takerUser, _, err := solana.FindProgramAddress([][]byte{
		[]byte("user"), takerPubkey[:], {0, 0},
	}, driftProgramId)
	if err != nil {
		panic(err)
	}

	fillerUser, _, err := solana.FindProgramAddress([][]byte{
		[]byte("user"), svmPubkey[:], {0, 0},
	}, driftProgramId)
	if err != nil {
		panic(err)
	}

	fmt.Println("taker user:", takerUser.String())
	msgTriggerStategy := &strategytypes.MsgTriggerStrategies{
		Sender: senderAddress.String(),
		Ids:    []string{"57a41e617e566f916ac6a4fc537cb01a7d77fb2d94e4e1a17cf94347c51539cb"},
		Inputs: [][]byte{
			[]byte(`{"fill_perp_market_order":{"taker_svm_address":"` + takerPubkey.String() + `","taker_order_id":"25","percent":"100"}}`),
		},
		Queries: []*astromeshtypes.FISQueryRequest{
			{
				Instructions: []*astromeshtypes.FISQueryInstruction{
					{
						Plane:   astromeshtypes.Plane_COSMOS,
						Action:  astromeshtypes.QueryAction_COSMOS_QUERY,
						Address: nil,
						Input: [][]byte{
							[]byte("/flux/svm/v1beta1/account_link/cosmos/" + senderAddress.String()),
						},
					},
					{
						Plane:   astromeshtypes.Plane_SVM,
						Action:  astromeshtypes.QueryAction_VM_QUERY,
						Address: nil,
						Input: [][]byte{
							fillerUser[:],
							takerUser[:],
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