package main

import (
	"context"
	signingv1beta1 "cosmossdk.io/api/cosmos/tx/signing/v1beta1"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/x/tx/signing"
	"fmt"
	"github.com/FluxNFTLabs/sdk-go/chain/modules/svm/golana"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ethsecp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// prepare info
	senderPrivKey := ethsecp256k1.PrivKey{Key: common.Hex2Bytes("88cbead91aee890d27bf06e003ade3d4e952427e88f88d31d61d3ef5e5d54305")}
	senderPubKey := senderPrivKey.PubKey()
	senderAddr := sdk.AccAddress(senderPubKey.Address().Bytes())
	programBufferPrivKey := ethsecp256k1.PrivKey{Key: common.Hex2Bytes("741de4f8988ea941d3ff0287911ca4074e62b7d45c991a51186455366f10b544")}
	programBufferPubkey := programBufferPrivKey.PubKey()
	programBufferAddr := sdk.AccAddress(programBufferPubkey.Address().Bytes())

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
	programBufferKeypair, err := solana.PrivateKeyFromBase58("3uEdHTmgWcU2iMp7msjZ777LCB1qvwW1WcFoJof5hzFzusg6wJ7XZZjA77scbwrrscSYEf1zuiqn2kod3q853b7A")
	if err != nil {
		panic(err)
	}
	upgradableLoaderId := solana.BPFLoaderUpgradeableProgramID

	// create instruction
	svmTxBuilder := solana.NewTransactionBuilder()
	ix := solana.NewInstruction(
		upgradableLoaderId,
		solana.AccountMetaSlice{
			{PublicKey: programBufferKeypair.PublicKey(), IsWritable: true, IsSigner: true},
			{PublicKey: senderId, IsWritable: true, IsSigner: true},
		},
		[]byte{0, 0, 0, 0},
	)

	svmTxBuilder.AddInstruction(ix)
	tx, err := svmTxBuilder.Build()
	msg := golana.ToCosmosMsg([]string{
		senderAddr.String(),
		programBufferAddr.String(),
	}, 1000000, tx)

	// init tx builder
	senderNum, senderSeq, err := clientCtx.AccountRetriever.GetAccountNumberSequence(clientCtx, senderAddr)
	if err != nil {
		panic(err)
	}
	programBufferNum, programBufferSeq, err := clientCtx.AccountRetriever.GetAccountNumberSequence(clientCtx, programBufferAddr)
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
	programBufferSigV2 := signingtypes.SignatureV2{
		PubKey: programBufferPubkey,
		Data: &signingtypes.SingleSignatureData{
			SignMode:  signingtypes.SignMode_SIGN_MODE_DIRECT,
			Signature: nil,
		},
		Sequence: programBufferSeq,
	}

	// build tx
	txBuilder.SetMsgs(msg)
	txBuilder.SetGasLimit(gasLimit)
	txBuilder.SetTimeoutHeight(timeoutHeight)
	txBuilder.SetMemo("abc")
	txBuilder.SetFeeAmount(fee)
	txBuilder.SetSignatures(senderSigV2, programBufferSigV2)

	// build sign data
	signerData := signing.SignerData{
		ChainID:       clientCtx.ChainID,
		AccountNumber: senderNum,
		Sequence:      senderSeq,
		//PubKey:        senderPubKey,
		Address: senderAddr.String(),
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
		AccountNumber: programBufferNum,
		Sequence:      programBufferSeq,
		//PubKey:        senderPubKey,
		Address: programBufferAddr.String(),
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
	programBufferSig, err := programBufferPrivKey.Sign(data)
	if err != nil {
		panic(err)
	}

	// set signatures again
	senderSigV2.Data.(*signingtypes.SingleSignatureData).Signature = senderSig
	programBufferSigV2.Data.(*signingtypes.SingleSignatureData).Signature = programBufferSig
	txBuilder.SetSignatures(senderSigV2, programBufferSigV2)

	// broadcast tx
	txBytes, err := clientCtx.TxConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		panic(err)
	}
	txRes, err := chainClient.SyncBroadcastSignedTx(txBytes)
	if err != nil {
		panic(err)
	}

	fmt.Println(txRes)
}
