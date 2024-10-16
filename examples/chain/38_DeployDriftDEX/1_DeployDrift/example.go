package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	_ "embed"

	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/FluxNFTLabs/sdk-go/client/svm"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ethsecp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	driftBinary  []byte
	driftPrivKey []byte
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
	clientCtx, _, err := chaintypes.NewClientContext(
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

	// load artifacts
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	driftBinary, err = os.ReadFile(dir + "/examples/chain/38_DeployDriftDEX/artifacts/drift.so")
	if err != nil {
		panic(err)
	}

	driftPrivKey, err = os.ReadFile(dir + "/examples/chain/38_DeployDriftDEX/artifacts/drift-keypair.json")
	if err != nil {
		panic(err)
	}

	cosmosPrivateKeys := []*ethsecp256k1.PrivKey{
		{Key: ethcommon.Hex2Bytes("88cbead91aee890d27bf06e003ade3d4e952427e88f88d31d61d3ef5e5d54305")},
		{Key: ethcommon.Hex2Bytes("88cbead91aee890d27bf06e003ade3d4e952427e88f88d31d61d3ef5e5d54306")},
		{Key: ethcommon.Hex2Bytes("88cbead91aee890d27bf06e003ade3d4e952427e88f88d31d61d3ef5e5d54307")},
	}
	cosmosAddrs := make([]sdk.AccAddress, len(cosmosPrivateKeys))
	for i, pk := range cosmosPrivateKeys {
		cosmosAddrs[i] = sdk.AccAddress(pk.PubKey().Address().Bytes())
	}

	// create missing cosmos accounts to sign upload message
	msgs := []sdk.Msg{}
	for _, addr := range cosmosAddrs[1:] {
		msgs = append(msgs, &banktypes.MsgSend{
			FromAddress: chainClient.FromAddress().String(),
			ToAddress:   addr.String(),
			Amount:      sdk.NewCoins(sdk.NewInt64Coin("lux", 100000000000000000)),
		})
	}
	_, err = chainClient.SyncBroadcastMsg(msgs...)
	if err != nil {
		panic(err)
	}

	// prepare svm accounts
	ownerSvmPrivKey := ed25519.GenPrivKeyFromSecret([]byte("owner"))
	ownerPubkey := solana.PublicKeyFromBytes(ownerSvmPrivKey.PubKey().Bytes())

	var programSvmPrivKeyBz []byte
	if err := json.Unmarshal(driftPrivKey, &programSvmPrivKeyBz); err != nil {
		panic(err)
	}

	programSvmPrivKey := &ed25519.PrivKey{Key: programSvmPrivKeyBz}
	programPubkey := solana.PublicKeyFromBytes(programSvmPrivKey.PubKey().Bytes())
	programBufferSvmPrivKey := ed25519.GenPrivKeyFromSecret([]byte("programBuffer"))
	programBufferPubkey := solana.PublicKeyFromBytes(programBufferSvmPrivKey.PubKey().Bytes())

	fmt.Println("=== linking accounts ===")
	ownerPubkey, _, err = svm.GetOrLinkSvmAccount(chainClient, clientCtx, cosmosPrivateKeys[0], ownerSvmPrivKey, 1000000000000000000)
	if err != nil {
		panic(err)
	}

	programPubkey, _, err = svm.GetOrLinkSvmAccount(chainClient, clientCtx, cosmosPrivateKeys[1], programSvmPrivKey, 0)
	if err != nil {
		panic(err)
	}

	programBufferPubkey, _, err = svm.GetOrLinkSvmAccount(chainClient, clientCtx, cosmosPrivateKeys[2], programBufferSvmPrivKey, 0)
	if err != nil {
		panic(err)
	}

	initAccountMsg := svm.CreateInitAccountsMsg(
		cosmosAddrs,
		len(driftBinary),
		ownerPubkey,
		programPubkey,
		programBufferPubkey,
	)

	uploadMsgs, err := svm.CreateProgramUploadMsgs(
		cosmosAddrs,
		ownerPubkey,
		programPubkey,
		programBufferPubkey,
		driftBinary,
	)
	if err != nil {
		panic(err)
	}

	fmt.Println("=== intialize accounts for uploading programs ===")
	signedTx, err := svm.BuildSignedTx(chainClient, []sdk.Msg{initAccountMsg}, cosmosPrivateKeys)
	if err != nil {
		panic(err)
	}

	txBytes, err := chainClient.ClientContext().TxConfig.TxEncoder()(signedTx)
	if err != nil {
		panic(err)
	}

	res, err := chainClient.SyncBroadcastSignedTx(txBytes)
	if err != nil {
		panic(err)
	}
	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)

	fmt.Println("=== start uploading program ===")
	fmt.Println("total txs required:", len(uploadMsgs))
	for i, uploadMsg := range uploadMsgs {
		fmt.Printf("uploading program part %dth\n", i+1)
		signedTx, err = svm.BuildSignedTx(chainClient, []sdk.Msg{uploadMsg}, cosmosPrivateKeys)
		if err != nil {
			panic(err)
		}

		txBytes, err = chainClient.ClientContext().TxConfig.TxEncoder()(signedTx)
		if err != nil {
			panic(err)
		}

		res, err = chainClient.SyncBroadcastSignedTx(txBytes)
		if err != nil {
			panic(err)
		}

		fmt.Println("tx hash:", res.TxResponse.TxHash)
		fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
		if res.TxResponse.Code != 0 {
			fmt.Println("err code:", res.TxResponse.Code, ", log:", res.TxResponse.RawLog)
		}
	}
	fmt.Println("✅ drift program deployed. program pubkey:", programPubkey.String())
}
