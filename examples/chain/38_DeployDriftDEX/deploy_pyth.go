package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	_ "embed"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/mr-tron/base58"

	sdkmath "cosmossdk.io/math"
	"github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	svmtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/FluxNFTLabs/sdk-go/client/svm"
	"github.com/FluxNFTLabs/sdk-go/examples/chain/38_DeployDriftDEX/pyth"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ethsecp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const MaxComputeBudget = 10000000

var (
	//go:embed artifacts/pyth.so
	pythBinary []byte

	//go:embed artifacts/pyth-keypair.json
	pythKeypair []byte
)

func BuildInitAccountsMsg(
	signerAddrs []sdk.AccAddress,
	programSize int,
	ownerPubkey solana.PublicKey,
	programPubkey solana.PublicKey,
	programBufferPubkey solana.PublicKey,
) *types.MsgTransaction {
	initTxBuilder := solana.NewTransactionBuilder()
	createAccountIx := system.NewCreateAccountInstruction(
		svmtypes.GetRentExemptLamportAmount(uint64(programSize)+48),
		uint64(programSize)+48,
		solana.BPFLoaderUpgradeableProgramID, ownerPubkey,
		programBufferPubkey,
	).Build()
	createProgramAccountIx := system.NewCreateAccountInstruction(
		svmtypes.GetRentExemptLamportAmount(36), 36,
		solana.BPFLoaderUpgradeableProgramID,
		ownerPubkey,
		programPubkey,
	).Build()
	initBufferAccountIx := solana.NewInstruction(
		solana.BPFLoaderUpgradeableProgramID,
		solana.AccountMetaSlice{
			&solana.AccountMeta{
				PublicKey:  programBufferPubkey,
				IsWritable: true,
				IsSigner:   true,
			},
			&solana.AccountMeta{
				PublicKey:  ownerPubkey,
				IsWritable: true,
				IsSigner:   true,
			},
		},
		[]byte{0, 0, 0, 0},
	)

	initTxBuilder.
		AddInstruction(createAccountIx).
		AddInstruction(createProgramAccountIx).
		AddInstruction(initBufferAccountIx)

	initTx, err := initTxBuilder.Build()
	if err != nil {
		panic(initTx)
	}

	signers := []string{}
	for _, acc := range signerAddrs {
		signers = append(signers, acc.String())
	}

	return svm.ToCosmosMsg(signers, MaxComputeBudget, initTx)
}

func BuildDeployMsg(
	signerAddrs []sdk.AccAddress,
	ownerPubkey solana.PublicKey,
	programPubkey solana.PublicKey,
	programBufferPubkey solana.PublicKey,
	programBz []byte,
) *types.MsgTransaction {
	// window size
	windowSize := 1200
	programSize := len(programBz)
	txBuilder := solana.NewTransactionBuilder()
	for idx := 0; idx < programSize; {
		end := idx + windowSize
		if end > programSize {
			end = programSize
		}

		codeSlice := programBz[idx:end]
		data := svm.MustMarshalIxData(svm.WriteBuffer{
			Offset: uint32(idx),
			Data:   codeSlice,
		})

		writeBufferIx := solana.NewInstruction(
			solana.BPFLoaderUpgradeableProgramID,
			solana.AccountMetaSlice{
				&solana.AccountMeta{
					PublicKey:  programBufferPubkey,
					IsWritable: true,
					IsSigner:   false,
				},
				&solana.AccountMeta{
					PublicKey:  ownerPubkey,
					IsWritable: true,
					IsSigner:   true,
				},
			},
			data,
		)
		txBuilder = txBuilder.AddInstruction(writeBufferIx)
		idx = end
	}

	data := svm.MustMarshalIxData(svm.DeployWithMaxDataLen{
		DataLen: uint64(len(programBz)) + 48,
	})

	programExecutablePubkey, _, err := solana.FindProgramAddress([][]byte{programPubkey[:]}, solana.BPFLoaderUpgradeableProgramID)
	if err != nil {
		panic(err)
	}

	deployIx := solana.NewInstruction(
		solana.BPFLoaderUpgradeableProgramID,
		solana.AccountMetaSlice{
			&solana.AccountMeta{
				PublicKey:  ownerPubkey,
				IsWritable: true,
				IsSigner:   true,
			},
			&solana.AccountMeta{
				PublicKey:  programExecutablePubkey,
				IsWritable: true,
				IsSigner:   false,
			},
			&solana.AccountMeta{
				PublicKey:  programPubkey,
				IsWritable: true,
				IsSigner:   false,
			},
			&solana.AccountMeta{
				PublicKey:  programBufferPubkey,
				IsWritable: true,
				IsSigner:   false,
			},
			&solana.AccountMeta{
				PublicKey:  solana.SysVarRentPubkey,
				IsWritable: false,
				IsSigner:   false,
			},
			&solana.AccountMeta{
				PublicKey:  solana.SysVarClockPubkey,
				IsWritable: false,
				IsSigner:   false,
			},
			&solana.AccountMeta{
				PublicKey:  solana.SystemProgramID,
				IsWritable: false,
				IsSigner:   false,
			},
			&solana.AccountMeta{
				PublicKey:  ownerPubkey,
				IsWritable: true,
				IsSigner:   true,
			},
		},
		data,
	)

	tx, err := txBuilder.AddInstruction(deployIx).Build()
	if err != nil {
		panic(err)
	}

	fmt.Println("number of instructions:", len(tx.Message.Instructions))

	signers := []string{}
	for _, acc := range signerAddrs {
		signers = append(signers, acc.String())
	}

	return svm.ToCosmosMsg(signers, MaxComputeBudget, tx)
}

func BuildSignedTx(
	chainClient chainclient.ChainClient,
	msgs []sdk.Msg,
	cosmosSignerKeys []*ethsecp256k1.PrivKey,
) (sdk.Tx, error) {
	cosmosAccs := []sdk.AccAddress{}
	cosmosPubkeys := []cryptotypes.PubKey{}
	for _, signer := range cosmosSignerKeys {
		cosmosPubkeys = append(cosmosPubkeys, signer.PubKey())
		acc := sdk.AccAddress(signer.PubKey().Address().Bytes())
		cosmosAccs = append(cosmosAccs, acc)
	}
	userNums := make([]uint64, len(cosmosSignerKeys))
	userSeqs := make([]uint64, len(cosmosSignerKeys))
	clientCtx := chainClient.ClientContext()
	// init tx builder
	for i, userAddr := range cosmosAccs {
		userNum, userSeq, err := clientCtx.AccountRetriever.GetAccountNumberSequence(clientCtx, userAddr)
		if err != nil {
			return nil, fmt.Errorf("get acc number err: %w", err)
		}

		userNums[i] = userNum
		userSeqs[i] = userSeq
	}

	txBuilder := clientCtx.TxConfig.NewTxBuilder()
	// prepare tx data
	timeoutHeight := uint64(19000000)
	gasLimit := uint64(300000000)
	gasPrice := sdkmath.NewIntFromUint64(500000000)
	fee := sdk.NewCoins(sdk.NewCoin("lux", sdkmath.NewIntFromUint64(gasLimit).Mul(gasPrice)))
	signatures := make([]signingtypes.SignatureV2, len(cosmosSignerKeys))

	for i := range cosmosPubkeys {
		userSigV2 := signingtypes.SignatureV2{
			PubKey: cosmosPubkeys[i],
			Data: &signingtypes.SingleSignatureData{
				SignMode:  signingtypes.SignMode_SIGN_MODE_DIRECT,
				Signature: nil,
			},
			Sequence: userSeqs[i],
		}
		signatures[i] = userSigV2
	}

	// build tx
	txBuilder.SetMsgs(msgs...)
	txBuilder.SetGasLimit(gasLimit)
	txBuilder.SetTimeoutHeight(timeoutHeight)
	txBuilder.SetFeeAmount(fee)
	txBuilder.SetSignatures(signatures...)

	// build and sign tx
	for i, pk := range cosmosSignerKeys {
		sig, err := tx.SignWithPrivKey(
			context.Background(),
			signingtypes.SignMode_SIGN_MODE_DIRECT,
			authsigning.SignerData{
				Address:       cosmosAccs[i].String(),
				ChainID:       clientCtx.ChainID,
				AccountNumber: userNums[i],
				Sequence:      userSeqs[i],
				PubKey:        pk.PubKey(),
			},
			txBuilder, pk, clientCtx.TxConfig, userSeqs[i],
		)
		if err != nil {
			panic(err)
		}

		signatures[i] = sig
	}

	// set signatures again
	txBuilder.SetSignatures(signatures...)
	// build sign data
	return txBuilder.GetTx(), nil
}

func LinkAccount(
	chainClient chainclient.ChainClient,
	clientCtx client.Context,
	cosmosPrivKey *ethsecp256k1.PrivKey,
	svmPrivKey *ed25519.PrivKey,
	luxAmount int64,
) (*txtypes.BroadcastTxResponse, error) {
	// prepare users
	userPubKey := cosmosPrivKey.PubKey()
	userAddr := sdk.AccAddress(userPubKey.Address().Bytes())

	// init link msg
	svmPubkey := svmPrivKey.PubKey()
	svmSig, err := svmPrivKey.Sign(userAddr.Bytes())
	if err != nil {
		return nil, fmt.Errorf("svm private key sign err: %w", err)
	}

	fmt.Println("gonna link:", userAddr.String(), "with", base58.Encode(svmPubkey.Bytes()))

	msg := &svmtypes.MsgLinkSVMAccount{
		Sender:       userAddr.String(),
		SvmPubkey:    svmPubkey.Bytes(),
		SvmSignature: svmSig[:],
		Amount:       sdk.NewInt64Coin("lux", luxAmount),
	}

	signedTx, err := BuildSignedTx(chainClient, []sdk.Msg{msg}, []*ethsecp256k1.PrivKey{cosmosPrivKey})
	if err != nil {
		return nil, err
	}

	txBytes, err := chainClient.ClientContext().TxConfig.TxEncoder()(signedTx)
	if err != nil {
		return nil, err
	}

	return chainClient.SyncBroadcastSignedTx(txBytes)
}

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
	if err := json.Unmarshal(pythKeypair, &programSvmPrivKeyBz); err != nil {
		panic(err)
	}
	programSvmPrivKey := &ed25519.PrivKey{Key: programSvmPrivKeyBz}
	programPubkey := solana.PublicKeyFromBytes(programSvmPrivKey.PubKey().Bytes())

	programBufferSvmPrivKey := ed25519.GenPrivKeyFromSecret([]byte("pyth_programBuffer"))
	programBufferPubkey := solana.PublicKeyFromBytes(programBufferSvmPrivKey.PubKey().Bytes())

	btcOraclePrivKey := ed25519.GenPrivKeyFromSecret([]byte("btc_oracle"))
	btcOraclePubkey := solana.PublicKeyFromBytes(btcOraclePrivKey.PubKey().Bytes())

	fmt.Println("pyth program buffer v1:", programBufferPubkey.String())
	res, err := LinkAccount(chainClient, clientCtx, cosmosPrivateKeys[0], ownerSvmPrivKey, 1000000000000000000)
	if err != nil {
		panic(err)
	}

	_, err = LinkAccount(chainClient, clientCtx, cosmosPrivateKeys[1], programSvmPrivKey, 0)
	if err != nil {
		panic(err)
	}

	_, err = LinkAccount(chainClient, clientCtx, cosmosPrivateKeys[2], programBufferSvmPrivKey, 0)
	if err != nil {
		panic(err)
	}

	// upload programs
	initAccountMsg := BuildInitAccountsMsg(
		cosmosAddrs,
		len(pythBinary),
		ownerPubkey,
		programPubkey,
		programBufferPubkey,
	)

	deployMsg := BuildDeployMsg(
		cosmosAddrs,
		ownerPubkey,
		programPubkey,
		programBufferPubkey,
		pythBinary,
	)

	fmt.Println("initializing accounts...")
	signedTx, err := BuildSignedTx(chainClient, []sdk.Msg{initAccountMsg}, cosmosPrivateKeys)
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
	signedTx, err = BuildSignedTx(chainClient, []sdk.Msg{deployMsg}, cosmosPrivateKeys)
	if err != nil {
		panic(err)
	}

	txBytes, err = chainClient.ClientContext().TxConfig.TxEncoder()(signedTx)
	if err != nil {
		panic(err)
	}
	fmt.Println("pyth program pubkey:", programPubkey.String())
	programExecutablePubkey, _, err := solana.FindProgramAddress([][]byte{programPubkey[:]}, solana.BPFLoaderUpgradeableProgramID)
	if err != nil {
		panic(err)
	}
	fmt.Println("program executable data pubkey:", programExecutablePubkey.String())

	res, err = chainClient.AsyncBroadcastSignedTx(txBytes)
	if err != nil {
		panic(err)
	}

	if res.TxResponse.Code != 0 {
		panic(fmt.Errorf("code: %d, err happen: %s", res.TxResponse.Code, res.TxResponse.RawLog))
	}

	fmt.Println("✅ pyth program deployed. tx hash:", res.TxResponse.TxHash)

	////////////////////////////
	/// initialize btc oracle
	///////////////////////////
	time.Sleep(2 * time.Second)
	oracleCosmosPrivKey := &ethsecp256k1.PrivKey{Key: ethcommon.Hex2Bytes("6bf7877e9bf7590d94b57d409b0fcf4cc80f9cd427bc212b1a2dd7ff6b6802e1")}
	oracleCosmosAddr := sdk.AccAddress(oracleCosmosPrivKey.PubKey().Address().Bytes())

	fmt.Println("Initialzing test BTC oracle account:", btcOraclePubkey.String())
	_, err = LinkAccount(chainClient, clientCtx, oracleCosmosPrivKey, btcOraclePrivKey, 0)
	if err != nil {
		panic(err)
	}

	oracleSize := uint64(3840) // deduce from Price struct
	createOracleAccountIx := system.NewCreateAccountInstruction(
		svmtypes.GetRentExemptLamportAmount(oracleSize),
		oracleSize,
		programPubkey,
		ownerPubkey,
		btcOraclePubkey,
	).Build()

	pyth.SetProgramID(programPubkey)
	initializeOracleIx := pyth.NewInitializeInstruction(
		65000_000000, 6, 1, btcOraclePubkey,
	).Build()

	initOracleTx, err := solana.NewTransactionBuilder().
		AddInstruction(createOracleAccountIx).
		AddInstruction(initializeOracleIx).
		Build()
	if err != nil {
		panic(err)
	}

	initOracleMsg := svm.ToCosmosMsg([]string{
		cosmosAddrs[0].String(),
		oracleCosmosAddr.String(),
	}, MaxComputeBudget, initOracleTx)

	oracleSignedTx, err := BuildSignedTx(
		chainClient, []sdk.Msg{initOracleMsg},
		[]*ethsecp256k1.PrivKey{
			cosmosPrivateKeys[0], oracleCosmosPrivKey,
		},
	)
	if err != nil {
		panic(err)
	}

	oracleTxBytes, err := chainClient.ClientContext().TxConfig.TxEncoder()(oracleSignedTx)
	if err != nil {
		panic(err)
	}

	res, err = chainClient.SyncBroadcastSignedTx(oracleTxBytes)
	if err != nil {
		panic(err)
	}
	fmt.Println("tx hash:", res.TxResponse.TxHash, "err:", res.TxResponse.RawLog)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
}
