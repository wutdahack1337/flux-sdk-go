package main

import (
	"context"

	signingv1beta1 "cosmossdk.io/api/cosmos/tx/signing/v1beta1"
	svmtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"

	"fmt"

	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/x/tx/signing"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ethsecp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	network := common.LoadNetwork("local", "")

	// prepare users
	senderPrivKey := ethsecp256k1.PrivKey{Key: ethcommon.Hex2Bytes("88cbead91aee890d27bf06e003ade3d4e952427e88f88d31d61d3ef5e5d54305")}
	senderPubKey := senderPrivKey.PubKey()
	senderAddr := sdk.AccAddress(senderPubKey.Address().Bytes())

	user2PrivKey := ethsecp256k1.PrivKey{Key: ethcommon.Hex2Bytes("741de4f8988ea941d3ff0287911ca4074e62b7d45c991a51186455366f10b544")}
	user2PubKey := user2PrivKey.PubKey()
	user2Addr := sdk.AccAddress(user2PubKey.Address().Bytes())

	user3PrivKey := ethsecp256k1.PrivKey{Key: ethcommon.Hex2Bytes("39a4c898dda351d54875d5ebb3e1c451189116faa556c3c04adc860dd1000608")}
	user3PubKey := user3PrivKey.PubKey()
	user3Addr := sdk.AccAddress(user3PubKey.Address().Bytes())

	// init grpc connection
	cc, err := grpc.Dial(network.ChainGrpcEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	// init client ctx
	clientCtx, _, err := chaintypes.NewClientContext("flux-1", "", nil)
	if err != nil {
		panic(err)
	}
	clientCtx = clientCtx.WithGRPCClient(cc)

	// init chain client
	chainClient, err := chainclient.NewChainClient(
		clientCtx,
	)

	senderId := solana.MustPublicKeyFromBase58("31kto8zBQ7c4mUhy2qnvBw6RGzhTFDr25HD2NNmpU8LW")
	programKeypair, err := solana.PrivateKeyFromBase58("afQDa9e9tZiKQPdxu6BBBkwgLRRLVZFykjvJZWLgKUpw685XzdhCK1qjRRT4FscXxDyKwjknfZVPQG75PaT7vzd")
	if err != nil {
		panic(err)
	}
	programBufferKeypair, err := solana.PrivateKeyFromBase58("3uEdHTmgWcU2iMp7msjZ777LCB1qvwW1WcFoJof5hzFzusg6wJ7XZZjA77scbwrrscSYEf1zuiqn2kod3q853b7A")
	if err != nil {
		panic(err)
	}
	upgradableLoaderId := solana.BPFLoaderUpgradeableProgramID

	// define space and rent-exempt lamports
	programIdSize := uint64(36)
	programIdLamports := svmtypes.GetRentExemptLamportAmount(programIdSize)
	programBufferIdSize := uint64(69000 + 48)
	programBufferLamports := svmtypes.GetRentExemptLamportAmount(programBufferIdSize)
	fmt.Println("program pubkey:", programKeypair.PublicKey().String(), programIdLamports)
	fmt.Println("program buffer pubkey:", programBufferKeypair.PublicKey().String(), programBufferLamports)

	// create instruction
	svmTxBuilder := solana.NewTransactionBuilder()
	svmTxBuilder.AddInstruction(system.NewCreateAccountInstruction(programIdLamports, programIdSize, upgradableLoaderId, senderId, programKeypair.PublicKey()).Build())
	svmTxBuilder.AddInstruction(system.NewCreateAccountInstruction(programBufferLamports, programBufferIdSize, upgradableLoaderId, senderId, programBufferKeypair.PublicKey()).Build())
	tx, err := svmTxBuilder.Build()
	msg := svmtypes.ToCosmosMsg([]string{
		senderAddr.String(),
		user2Addr.String(),
		user3Addr.String()},
		1000000,
		tx,
	)

	// init tx builder
	senderNum, senderSeq, err := clientCtx.AccountRetriever.GetAccountNumberSequence(clientCtx, senderAddr)
	if err != nil {
		panic(err)
	}
	user2Num, user2Seq, err := clientCtx.AccountRetriever.GetAccountNumberSequence(clientCtx, user2Addr)
	if err != nil {
		panic(err)
	}
	user3Num, user3Seq, err := clientCtx.AccountRetriever.GetAccountNumberSequence(clientCtx, user3Addr)
	if err != nil {
		panic(err)
	}

	txConfig := chaintypes.NewTxConfig([]signingtypes.SignMode{signingtypes.SignMode_SIGN_MODE_DIRECT})
	txBuilder := txConfig.NewTxBuilder()

	// prepare tx data
	timeoutHeight := uint64(19000)
	gasLimit := uint64(3000000)
	gasPrice := sdkmath.NewIntFromUint64(500000000)
	fee := []sdk.Coin{{
		Denom:  "lux",
		Amount: sdkmath.NewIntFromUint64(gasLimit).Mul(gasPrice),
	}}
	senderSigV2 := signingtypes.SignatureV2{
		PubKey: senderPubKey,
		Data: &signingtypes.SingleSignatureData{
			SignMode:  signingtypes.SignMode_SIGN_MODE_DIRECT,
			Signature: nil,
		},
		Sequence: senderSeq,
	}
	user2SigV2 := signingtypes.SignatureV2{
		PubKey: user2PubKey,
		Data: &signingtypes.SingleSignatureData{
			SignMode:  signingtypes.SignMode_SIGN_MODE_DIRECT,
			Signature: nil,
		},
		Sequence: user2Seq,
	}
	user3SigV2 := signingtypes.SignatureV2{
		PubKey: user3PubKey,
		Data: &signingtypes.SingleSignatureData{
			SignMode:  signingtypes.SignMode_SIGN_MODE_DIRECT,
			Signature: nil,
		},
		Sequence: user3Seq,
	}

	// build tx
	txBuilder.SetMsgs(msg)
	txBuilder.SetGasLimit(gasLimit)
	txBuilder.SetTimeoutHeight(timeoutHeight)
	txBuilder.SetMemo("abc")
	txBuilder.SetFeeAmount(fee)
	txBuilder.SetSignatures(senderSigV2, user2SigV2, user3SigV2)

	// build sign data
	signerData := signing.SignerData{
		ChainID:       clientCtx.ChainID,
		AccountNumber: senderNum,
		Sequence:      senderSeq,
		Address:       senderAddr.String(),
	}
	data, err := txConfig.SignModeHandler().GetSignBytes(
		context.Background(),
		signingv1beta1.SignMode_SIGN_MODE_DIRECT,
		signerData,
		txBuilder.(authsigning.V2AdaptableTx).GetSigningTxData(),
	)
	if err != nil {
		panic(err)
	}
	senderSig, err := senderPrivKey.Sign(data)
	if err != nil {
		panic(err)
	}

	signerData = signing.SignerData{
		ChainID:       clientCtx.ChainID,
		AccountNumber: user2Num,
		Sequence:      user2Seq,
		Address:       user2Addr.String(),
	}
	data, err = txConfig.SignModeHandler().GetSignBytes(
		context.Background(),
		signingv1beta1.SignMode_SIGN_MODE_DIRECT,
		signerData,
		txBuilder.(authsigning.V2AdaptableTx).GetSigningTxData(),
	)
	if err != nil {
		panic(err)
	}
	user2Sig, err := user2PrivKey.Sign(data)
	if err != nil {
		panic(err)
	}

	signerData = signing.SignerData{
		ChainID:       clientCtx.ChainID,
		AccountNumber: user3Num,
		Sequence:      user3Seq,
		Address:       user3Addr.String(),
	}
	data, err = txConfig.SignModeHandler().GetSignBytes(
		context.Background(),
		signingv1beta1.SignMode_SIGN_MODE_DIRECT,
		signerData,
		txBuilder.(authsigning.V2AdaptableTx).GetSigningTxData(),
	)
	if err != nil {
		panic(err)
	}
	user3Sig, err := user3PrivKey.Sign(data)
	if err != nil {
		panic(err)
	}

	// set signatures again
	senderSigV2.Data.(*signingtypes.SingleSignatureData).Signature = senderSig
	user2SigV2.Data.(*signingtypes.SingleSignatureData).Signature = user2Sig
	user3SigV2.Data.(*signingtypes.SingleSignatureData).Signature = user3Sig
	txBuilder.SetSignatures(senderSigV2, user2SigV2, user3SigV2)

	// broadcast tx
	txBytes, err := clientCtx.TxConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		panic(err)
	}
	txRes, err := chainClient.SyncBroadcastSignedTx(txBytes)
	if err != nil {
		panic(err)
	}

	fmt.Println("txHash:", txRes.TxResponse.TxHash)
	fmt.Println("gas used/want:", txRes.TxResponse.GasUsed, "/", txRes.TxResponse.GasWanted)
}
