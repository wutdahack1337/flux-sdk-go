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
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const MaxComputeBudget = 10000000

var (
	pythBinary []byte

	pythPrivKey []byte
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

	pythBinary, err = os.ReadFile(dir + "/examples/chain/38_DeployDriftDEX/artifacts/pyth.so")
	if err != nil {
		panic(err)
	}

	pythPrivKey, err = os.ReadFile(dir + "/examples/chain/38_DeployDriftDEX/artifacts/pyth-keypair.json")
	if err != nil {
		panic(err)
	}

	// prepare cosmos accounts
	cosmosPrivateKeys := []*ethsecp256k1.PrivKey{
		{Key: ethcommon.Hex2Bytes("88cbead91aee890d27bf06e003ade3d4e952427e88f88d31d61d3ef5e5d54305")},
		{Key: ethcommon.Hex2Bytes("c25e5cccd433d2c97971eaa6cfe92ea05771dc05b984c62464ab580f16a905e1")},
		{Key: ethcommon.Hex2Bytes("26fc2228a05e83d443066f643754d5837a2b39b5783d804eb125b936d630204b")},
	}
	cosmosAddrs := make([]sdk.AccAddress, len(cosmosPrivateKeys))
	for i, pk := range cosmosPrivateKeys {
		cosmosAddrs[i] = sdk.AccAddress(pk.PubKey().Address().Bytes())
	}

	// prepare svm accounts
	ownerSvmPrivKey := ed25519.GenPrivKeyFromSecret([]byte("owner"))
	ownerPubkey := solana.PublicKeyFromBytes(ownerSvmPrivKey.PubKey().Bytes())

	var programSvmPrivKeyBz []byte
	if err := json.Unmarshal(pythPrivKey, &programSvmPrivKeyBz); err != nil {
		panic(err)
	}
	programSvmPrivKey := &ed25519.PrivKey{Key: programSvmPrivKeyBz}
	programPubkey := solana.PublicKeyFromBytes(programSvmPrivKey.PubKey().Bytes())

	programBufferSvmPrivKey := ed25519.GenPrivKeyFromSecret([]byte("pyth_programBuffer"))
	programBufferPubkey := solana.PublicKeyFromBytes(programBufferSvmPrivKey.PubKey().Bytes())

	res, err := svm.LinkAccount(chainClient, clientCtx, cosmosPrivateKeys[0], ownerSvmPrivKey, 1000000000000000000)
	if err != nil {
		panic(err)
	}

	_, err = svm.LinkAccount(chainClient, clientCtx, cosmosPrivateKeys[1], programSvmPrivKey, 0)
	if err != nil {
		panic(err)
	}

	_, err = svm.LinkAccount(chainClient, clientCtx, cosmosPrivateKeys[2], programBufferSvmPrivKey, 0)
	if err != nil {
		panic(err)
	}

	// upload programs
	initAccountMsg := svm.CreateInitAccountsMsg(
		cosmosAddrs,
		len(pythBinary),
		ownerPubkey,
		programPubkey,
		programBufferPubkey,
	)

	uploadMsgs, err := svm.CreateProgramUploadMsgs(
		cosmosAddrs,
		ownerPubkey,
		programPubkey,
		programBufferPubkey,
		pythBinary,
	)
	if err != nil {
		panic(err)
	}

	fmt.Println("initializing accounts...")
	signedTx, err := svm.BuildSignedTx(chainClient, []sdk.Msg{initAccountMsg}, cosmosPrivateKeys)
	if err != nil {
		panic(err)
	}

	txBytes, err := chainClient.ClientContext().TxConfig.TxEncoder()(signedTx)
	if err != nil {
		panic(err)
	}

	res, err = chainClient.SyncBroadcastSignedTx(txBytes)
	if err != nil {
		panic(err)
	}

	fmt.Println("tx hash:", res.TxResponse.TxHash, res.TxResponse.RawLog)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)

	// deploy program
	fmt.Println("total txs required for uploading program:", len(uploadMsgs), ". Uploading...")
	for i, uploadMsg := range uploadMsgs {
		fmt.Printf("=== uploading program part %dth ===\n", i+1)
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

	fmt.Println("pyth program pubkey:", programPubkey.String())
	programExecutablePubkey, _, err := solana.FindProgramAddress([][]byte{programPubkey[:]}, solana.BPFLoaderUpgradeableProgramID)
	if err != nil {
		panic(err)
	}
	fmt.Println("program executable data pubkey:", programExecutablePubkey.String())

	if res.TxResponse.Code != 0 {
		panic(fmt.Errorf("code: %d, err happen: %s", res.TxResponse.Code, res.TxResponse.RawLog))
	}

	fmt.Println("✅ pyth program deployed. tx hash:", res.TxResponse.TxHash)
}
