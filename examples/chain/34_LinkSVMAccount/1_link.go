package main

import (
	"fmt"
	svmtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"os"
	"strings"
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
	svmPrivKey := ed25519.PrivKey{Key: ethcommon.Hex2Bytes("bae23479c25ea273d45649ebb40463fb1df36ad395fa5e8c5c4469e64a75c3c61de900fca14b5219e43fb7777a887d81edec987e07b0895b9c1a097384c6c8cf")}
	svmPubKey := svmPrivKey.PubKey() // 31kto8zBQ7c4mUhy2qnvBw6RGzhTFDr25HD2NNmpU8LW
	svmSig, err := svmPrivKey.Sign(senderAddress.Bytes())
	if err != nil {
		panic(err)
	}
	msg := &svmtypes.MsgLinkSVMAccount{
		Sender:       senderAddress.String(),
		SvmPubkey:    svmPubKey.Bytes(),
		SvmSignature: svmSig,
	}

	//AsyncBroadcastMsg, SyncBroadcastMsg, QueueBroadcastMsg
	res, err := chainClient.SyncBroadcastMsg(msg)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(res)
}
