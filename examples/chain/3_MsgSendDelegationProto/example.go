package main

import (
	"context"
	signingv1beta1 "cosmossdk.io/api/cosmos/tx/signing/v1beta1"

	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/x/tx/signing"
	"fmt"
	secp256k1 "github.com/FluxNFTLabs/sdk-go/chain/crypto/ethsecp256k1"
	types "github.com/FluxNFTLabs/sdk-go/chain/indexer/web3gw"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// prepare info
	senderPrivKey := secp256k1.PrivKey{Key: common.Hex2Bytes("88CBEAD91AEE890D27BF06E003ADE3D4E952427E88F88D31D61D3EF5E5D54305")}
	senderPubKey := senderPrivKey.PubKey()
	senderAddr := sdk.AccAddress(senderPubKey.Address().Bytes())
	receiverAddr := sdk.MustAccAddressFromBech32("lux1jcltmuhplrdcwp7stlr4hlhlhgd4htqhu86cqx")

	// init web3gw client
	cc, err := grpc.Dial("localhost:4444", grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer cc.Close()
	if err != nil {
		panic(err)
	}
	client := types.NewWeb3GWClient(cc)

	// get fee payer metadata
	metadata, err := client.GetMetaData(context.Background(), &types.GetMetaDataRequest{})
	if err != nil {
		panic(err)
	}
	feePayerAddr := sdk.MustAccAddressFromBech32(metadata.Address)
	feePayerPubKey := cryptotypes.PubKey(&secp256k1.PubKey{Key: metadata.Pubkey})

	// init client ctx
	clientCtx, err := chaintypes.NewClientContext("flux-1", "", nil)
	if err != nil {
		panic(err)
	}
	tmClient, err := rpchttp.New("http://localhost:26657", "/websocket")
	if err != nil {
		panic(err)
	}
	clientCtx = clientCtx.WithClient(tmClient)

	// init msg
	msg := &banktypes.MsgSend{
		FromAddress: senderAddr.String(),
		ToAddress:   receiverAddr.String(),
		Amount: sdk.Coins{
			{Denom: "lux", Amount: sdkmath.NewIntFromUint64(77)},
		},
	}

	// init tx builder
	senderNum, senderSeq, err := clientCtx.AccountRetriever.GetAccountNumberSequence(clientCtx, senderAddr)
	if err != nil {
		panic(err)
	}
	feePayerNum, feePayerSeq, err := clientCtx.AccountRetriever.GetAccountNumberSequence(clientCtx, feePayerAddr)
	if err != nil {
		panic(err)
	}

	txConfig := chaintypes.NewTxConfig([]signingtypes.SignMode{signingtypes.SignMode_SIGN_MODE_DIRECT})
	extTxBuilder, ok := txConfig.NewTxBuilder().(authtx.ExtensionOptionsTxBuilder)
	if !ok {
		panic("cannot cast txBuilder")
	}

	// prepare tx data
	timeoutHeight := uint64(19000)
	gasLimit := uint64(200000)
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
	feePayerSigV2 := signingtypes.SignatureV2{
		PubKey: feePayerPubKey,
		Data: &signingtypes.SingleSignatureData{
			SignMode:  signingtypes.SignMode_SIGN_MODE_DIRECT,
			Signature: nil,
		},
		Sequence: feePayerSeq,
	}

	// build tx
	extTxBuilder.SetMsgs(msg)
	extTxBuilder.SetGasLimit(gasLimit)
	extTxBuilder.SetTimeoutHeight(timeoutHeight)
	extTxBuilder.SetMemo("abc")
	extTxBuilder.SetFeePayer(feePayerAddr)
	extTxBuilder.SetFeeAmount(fee)
	extTxBuilder.SetSignatures(senderSigV2, feePayerSigV2)

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
		extTxBuilder.(authsigning.V2AdaptableTx).GetSigningTxData(),
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
		AccountNumber: feePayerNum,
		Sequence:      feePayerSeq,
		//PubKey:        feePayerPubKey,
		Address: feePayerAddr.String(),
	}
	data, err = txConfig.SignModeHandler().GetSignBytes(
		context.Background(),
		signingv1beta1.SignMode_SIGN_MODE_DIRECT,
		signerData,
		extTxBuilder.(authsigning.V2AdaptableTx).GetSigningTxData(),
	)

	// get fee payer sig
	res, err := client.SignProto(context.Background(), &types.SignProtoRequest{Data: data})
	if err != nil {
		panic(err)
	}

	// double check gateway hash
	hash := ethcrypto.Keccak256Hash(data).Bytes()
	if common.Bytes2Hex(hash) != common.Bytes2Hex(res.Hash) {
		panic("mismatched typed data hash from fee payer")
	}

	// set signatures again
	senderSigV2.Data.(*signingtypes.SingleSignatureData).Signature = senderSig
	feePayerSigV2.Data.(*signingtypes.SingleSignatureData).Signature = res.Signature
	extTxBuilder.SetSignatures(senderSigV2, feePayerSigV2)

	// broadcast tx
	txBytes, err := clientCtx.TxConfig.TxEncoder()(extTxBuilder.GetTx())
	if err != nil {
		panic(err)
	}
	txRes, err := clientCtx.BroadcastTxSync(txBytes)
	if err != nil {
		panic(err)
	}

	fmt.Println(txRes)
}
