package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/FluxNFTLabs/sdk-go/chain/modules/svm/golana"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"

	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
)

func main() {
	network := common.LoadNetwork("local", "")
	kr, err := keyring.New(
		"fluxd",
		"file",
		os.Getenv("HOME")+"/.fluxd",
		strings.NewReader("12345678\n"),
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

	// accounts
	senderId := solana.PublicKeyFromBytes(senderAddress.Bytes())
	programId := solana.MustPublicKeyFromBase58("8BTVbEdAFbqsEsjngmaMByn1m9j8jDFtEFFusEwGeMZY")
	programBufferId := solana.NewWallet().PublicKey()
	upgradableLoaderId := solana.BPFLoaderUpgradeableProgramID

	// build msg
	initTxBuilder := solana.NewTransactionBuilder()
	createAccountIx := system.NewCreateAccountInstruction(0, uint64(69000)+48,
		solana.BPFLoaderUpgradeableProgramID, senderId, programBufferId,
	).Build()
	createProgramAccountIx := system.NewCreateAccountInstruction(
		0, 36,
		upgradableLoaderId, senderId, programId,
	).Build()
	initBufferAccountIx := solana.NewInstruction(
		solana.BPFLoaderUpgradeableProgramID,
		solana.AccountMetaSlice{
			{PublicKey: programBufferId, IsWritable: true, IsSigner: true},
			{PublicKey: senderId, IsWritable: true, IsSigner: true},
		},
		[]byte{0, 0, 0, 0},
	)
	initTxBuilder.AddInstruction(createAccountIx).
		AddInstruction(createProgramAccountIx).
		AddInstruction(initBufferAccountIx)

	tx, err := initTxBuilder.Build()
	msg := golana.ToCosmosMsg([]string{senderAddress.String()}, 1000000, tx)

	// broadcast msg
	res, err := chainClient.SyncBroadcastSvmMsg(msg)

	fmt.Println(res, err)
}
