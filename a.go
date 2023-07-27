package main

import (
	"fmt"
	eip712 "github.com/FluxNFTLabs/sdk-go/chain/app/ante"
	"github.com/FluxNFTLabs/sdk-go/chain/app/ante/typeddata"
	secp256k1 "github.com/FluxNFTLabs/sdk-go/chain/crypto/ethsecp256k1"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

type Fee struct {
	Amount   sdk.Coins `json:"amount"`
	FeePayer string    `json:"feePayer"`
	Gas      string    `json:"gas"`
}

func main() {
	clientCtx, err := chaintypes.NewClientContext("flux-1", "", nil)
	if err != nil {
		panic(err)
	}
	tmClient, err := rpchttp.New("http://localhost:26657", "/websocket")
	if err != nil {
		panic(err)
	}
	clientCtx = clientCtx.WithClient(tmClient)

	senderPrivKey := secp256k1.PrivKey{Key: common.Hex2Bytes("88CBEAD91AEE890D27BF06E003ADE3D4E952427E88F88D31D61D3EF5E5D54305")}
	senderPubKey := senderPrivKey.PubKey()
	senderAddr := sdk.AccAddress(senderPubKey.Address().Bytes())

	receiverAddr := "lux1jcltmuhplrdcwp7stlr4hlhlhgd4htqhu86cqx"

	feePayerPrivKey := secp256k1.PrivKey{Key: common.Hex2Bytes("39A4C898DDA351D54875D5EBB3E1C451189116FAA556C3C04ADC860DD1000608")}
	feePayerPubKey := feePayerPrivKey.PubKey()
	feePayerAddr := sdk.AccAddress(feePayerPubKey.Address().Bytes())

	// init msg
	msg := &banktypes.MsgSend{
		FromAddress: senderAddr.String(),
		ToAddress:   receiverAddr,
		Amount: sdk.Coins{
			{Denom: "lux", Amount: sdk.NewIntFromUint64(77)},
		},
	}

	protoCodec := codec.NewProtoCodec(chaintypes.RegisterTypes())

	accNum, accSeq, err := clientCtx.AccountRetriever.GetAccountNumberSequence(clientCtx, senderAddr)
	if err != nil {
		panic(err)
	}

	txConfig := chaintypes.NewTxConfig([]signingtypes.SignMode{
		signingtypes.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
		signingtypes.SignMode_SIGN_MODE_DIRECT,
	})
	txBuilder := txConfig.NewTxBuilder()

	timeoutHeight := uint64(19000)
	gasLimit := uint64(200000)
	gasPrice := sdk.NewIntFromUint64(500000)
	fee := []sdk.Coin{{
		Denom:  "lux",
		Amount: sdk.NewIntFromUint64(gasLimit).Mul(gasPrice),
	}}

	txBuilder.SetMsgs(msg)
	txBuilder.SetGasLimit(gasLimit)
	txBuilder.SetFeeAmount(fee)
	txBuilder.SetTimeoutHeight(timeoutHeight)
	txBuilder.SetMemo("abc")

	signerData := authsigning.SignerData{
		Address:       senderAddr.String(),
		ChainID:       clientCtx.ChainID,
		AccountNumber: accNum,
		Sequence:      accSeq,
	}

	data, err := txConfig.SignModeHandler().GetSignBytes(
		signingtypes.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
		signerData,
		txBuilder.GetTx(),
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

	typedDataHash, err := typeddata.ComputeTypedDataHash(typedData)
	if err != nil {
		panic(err)
	}

	extTxBuilder, ok := txConfig.NewTxBuilder().(authtx.ExtensionOptionsTxBuilder)
	if !ok {
		panic("cannot cast txBuilder")
	}

	// prepare ext tx bulder data
	feePayerSig, err := ethcrypto.Sign(typedDataHash, feePayerPrivKey.ToECDSA())
	if err != nil {
		panic(err)
	}
	extOpts := &chaintypes.ExtensionOptionsWeb3Tx{
		TypedDataChainID: 1,
		FeePayer:         feePayerAddr.String(),
		FeePayerSig:      feePayerSig,
	}
	extOptsAny, err := codectypes.NewAnyWithValue(extOpts)
	if err != nil {
		panic(err)
	}

	senderSig, err := ethcrypto.Sign(typedDataHash, senderPrivKey.ToECDSA())
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

	// construct tx
	extTxBuilder.SetMsgs(msg)
	extTxBuilder.SetGasLimit(gasLimit)
	extTxBuilder.SetFeeAmount(fee)
	extTxBuilder.SetTimeoutHeight(timeoutHeight)
	extTxBuilder.SetMemo("abc")
	extTxBuilder.SetSignatures(sig)
	extTxBuilder.SetExtensionOptions(extOptsAny)

	txBytes, err := clientCtx.TxConfig.TxEncoder()(extTxBuilder.GetTx())
	if err != nil {
		panic(err)
	}

	txRes, err := clientCtx.BroadcastTxAsync(txBytes)
	if err != nil {
		panic(err)
	}

	fmt.Println(txRes)
}
