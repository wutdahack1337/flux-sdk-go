package main

import (
	"context"
	signingv1beta1 "cosmossdk.io/api/cosmos/tx/signing/v1beta1"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/x/tx/signing"
	"encoding/json"
	"fmt"
	eip712 "github.com/FluxNFTLabs/sdk-go/chain/app/ante"
	"github.com/FluxNFTLabs/sdk-go/chain/app/ante/typeddata"
	types "github.com/FluxNFTLabs/sdk-go/chain/indexer/web3gw"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
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

	// init grpc connection
	cc, err = grpc.Dial(network.ChainGrpcEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
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

	// init msg
	msg := &banktypes.MsgSend{
		FromAddress: senderAddr.String(),
		ToAddress:   receiverAddr.String(),
		Amount: sdk.Coins{
			{Denom: "lux", Amount: sdkmath.NewIntFromUint64(77)},
		},
	}

	// init tx builder
	protoCodec := codec.NewProtoCodec(chaintypes.RegisterTypes())
	accNum, accSeq, err := clientCtx.AccountRetriever.GetAccountNumberSequence(clientCtx, senderAddr)
	if err != nil {
		panic(err)
	}

	txConfig := chaintypes.NewTxConfig([]signingtypes.SignMode{signingtypes.SignMode_SIGN_MODE_LEGACY_AMINO_JSON})
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

	// build tx
	extTxBuilder.SetMsgs(msg)
	extTxBuilder.SetGasLimit(gasLimit)
	extTxBuilder.SetFeeAmount(fee)
	extTxBuilder.SetTimeoutHeight(timeoutHeight)
	extTxBuilder.SetMemo("abc")

	// build typed data
	signerData := signing.SignerData{
		Address:       senderAddr.String(),
		ChainID:       clientCtx.ChainID,
		AccountNumber: accNum,
		Sequence:      accSeq,
	}
	data, err := txConfig.SignModeHandler().GetSignBytes(
		context.Background(),
		signingv1beta1.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
		signerData,
		extTxBuilder.(authsigning.V2AdaptableTx).GetSigningTxData(),
	)
	if err != nil {
		panic(err)
	}

	feeDelegationOptions := &eip712.FeeDelegationOptions{
		FeePayer: feePayerAddr,
	}

	typedData, err := eip712.WrapTxToEIP712(
		protoCodec,
		1,
		msg,
		data,
		feeDelegationOptions,
	)
	if err != nil {
		panic(err)
	}

	// get fee payer sig
	typedDataBytes, err := json.Marshal(typedData)
	if err != nil {
		panic(err)
	}

	res, err := client.SignJSON(context.Background(), &types.SignJSONRequest{Data: typedDataBytes})
	if err != nil {
		panic(err)
	}

	// double check gateway hash
	typedDataHash, err := typeddata.ComputeTypedDataHash(typedData)
	if err != nil {
		panic(err)
	}

	if common.Bytes2Hex(typedDataHash) != common.Bytes2Hex(res.Hash) {
		panic("mismatched typed data hash from fee payer")
	}

	// build sender sig
	senderEthPk, _ := ethcrypto.ToECDSA(senderPrivKey.Bytes())
	senderSig, err := ethcrypto.Sign(typedDataHash, senderEthPk)
	if err != nil {
		panic(err)
	}
	sig := signingtypes.SignatureV2{
		PubKey: senderPubKey,
		Data: &signingtypes.SingleSignatureData{
			SignMode:  signingtypes.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
			Signature: senderSig,
		},
		Sequence: accSeq,
	}
	extTxBuilder.SetSignatures(sig)

	// add extension opts with fee payer sig
	extOpts := &chaintypes.ExtensionOptionsWeb3Tx{
		TypedDataChainID: 1,
		FeePayer:         feePayerAddr.String(),
		FeePayerSig:      res.Signature,
	}
	extOptsAny, err := codectypes.NewAnyWithValue(extOpts)
	if err != nil {
		panic(err)
	}
	extTxBuilder.SetExtensionOptions(extOptsAny)

	// broadcast tx
	txBytes, err := clientCtx.TxConfig.TxEncoder()(extTxBuilder.GetTx())
	if err != nil {
		panic(err)
	}
	txRes, err := chainClient.SyncBroadcastSignedTx(txBytes)
	if err != nil {
		panic(err)
	}

	fmt.Println(txRes)
}
