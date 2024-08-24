package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/FluxNFTLabs/sdk-go/examples/chain/38_DeployDriftDEX/drift"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	driftProgramId = solana.MustPublicKeyFromBase58("FLR3mfYrMZUnhqEadNJVwjUhjX8ky9vE9qTtDmkK4vwC")
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

	isSvmLinked, svmPubkey, err := chainClient.GetSVMAccountLink(context.Background(), senderAddress)
	if err != nil {
		panic(err)
	}

	if !isSvmLinked {
		panic(fmt.Errorf("no svm account found"))
	}

	user, _, err := solana.FindProgramAddress([][]byte{
		[]byte("user"), svmPubkey[:], []byte{0, 0},
	}, driftProgramId)
	if err != nil {
		panic(err)
	}

	acc, err := chainClient.GetSvmAccount(context.Background(), user.String())
	if err != nil {
		panic(err)
	}

	var userStruct drift.User
	err = userStruct.UnmarshalWithDecoder(bin.NewBinDecoder(acc.Account.Data))
	if err != nil {
		panic(err)
	}

	fmt.Println("user pda:", user.String())
	for _, o := range userStruct.Orders {
		if o.OrderId > 0 {
			bz, _ := json.MarshalIndent(o, "", "  ")
			fmt.Println(string(bz))
		}
	}
}
