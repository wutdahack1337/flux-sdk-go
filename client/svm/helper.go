package svm

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"

	sdkmath "cosmossdk.io/math"
	"github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	svmtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	pyth "github.com/FluxNFTLabs/sdk-go/client/svm/drift_pyth"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ethsecp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	ethcommon "github.com/ethereum/go-ethereum/common"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
)

var (
	MaxComputeBudget   = uint64(10_000_000)
	ProgramAccountSize = 36
)

type WriteBuffer struct {
	Offset uint32
	Data   []byte
}

func MarshalIxData(s interface{}) ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := bin.NewBinEncoder(buf)
	err := enc.Encode(s)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func MustMarshalIxData(s interface{}) []byte {
	bz, err := MarshalIxData(s)
	if err != nil {
		panic(err)
	}

	return bz
}

func (inst WriteBuffer) MarshalWithEncoder(encoder *bin.Encoder) error {
	if err := encoder.WriteInt32(int32(1), binary.LittleEndian); err != nil {
		return err
	}

	if err := encoder.WriteUint32(inst.Offset, binary.LittleEndian); err != nil {
		return err
	}

	if err := encoder.WriteUint64(uint64(len(inst.Data)), binary.LittleEndian); err != nil {
		return err
	}

	if err := encoder.WriteBytes(inst.Data, false); err != nil {
		return err
	}

	return nil
}

type DeployWithMaxDataLen struct {
	DataLen uint64
}

func (inst DeployWithMaxDataLen) MarshalWithEncoder(encoder *bin.Encoder) error {
	if err := encoder.WriteInt32(int32(2), binary.LittleEndian); err != nil {
		return err
	}

	return encoder.WriteUint64(inst.DataLen, binary.LittleEndian)
}

// Find associated token account, panic when not found (not likely to happen)
func MustFindAta(
	wallet, tokenProgram, mint, ataProgram solana.PublicKey,
) solana.PublicKey {
	ata, _, err := solana.FindProgramAddress([][]byte{
		wallet[:], tokenProgram[:], mint[:],
	}, ataProgram)
	if err != nil {
		panic(err)
	}

	return ata
}

func CreateInitAccountsMsg(
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

	return ToCosmosMsg(signers, MaxComputeBudget, initTx)
}

func CreateProgramUploadMsgs(
	signerAddrs []sdk.AccAddress,
	ownerPubkey solana.PublicKey,
	programPubkey solana.PublicKey,
	programBufferPubkey solana.PublicKey,
	programBz []byte,
) (res []*types.MsgTransaction, err error) {
	signers := []string{}
	for _, acc := range signerAddrs {
		signers = append(signers, acc.String())
	}

	instructionSize := 1200
	instructionsPerTx := 750
	programSize := len(programBz)
	txBuilder := solana.NewTransactionBuilder()

	instructionCount := 0
	for idx := 0; idx < programSize; {
		end := idx + instructionSize
		if end > programSize {
			end = programSize
		}

		codeSlice := programBz[idx:end]
		data := MustMarshalIxData(WriteBuffer{
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

		instructionCount++
		if instructionCount == instructionsPerTx {
			instructionCount = 0
			tx, err := txBuilder.Build()
			if err != nil {
				return nil, fmt.Errorf("solana tx build err: %w", err)
			}

			res = append(res, ToCosmosMsg(signers, MaxComputeBudget, tx))
			txBuilder = solana.NewTransactionBuilder()
		}
	}

	data, err := MarshalIxData(DeployWithMaxDataLen{
		DataLen: uint64(len(programBz)) + 48,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal DeployWithMaxDataLen err: %w", err)
	}

	programExecutablePubkey, _, err := solana.FindProgramAddress([][]byte{programPubkey[:]}, solana.BPFLoaderUpgradeableProgramID)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	res = append(res, ToCosmosMsg(signers, MaxComputeBudget, tx))
	return res, nil
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

	for i, userAddr := range cosmosAccs {
		userNum, userSeq, err := clientCtx.AccountRetriever.GetAccountNumberSequence(clientCtx, userAddr)
		if err != nil {
			return nil, fmt.Errorf("get acc number err: %w", err)
		}

		userNums[i] = userNum
		userSeqs[i] = userSeq
	}

	txBuilder := clientCtx.TxConfig.NewTxBuilder()
	timeoutHeight := uint64(19000000)
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
	txBuilder.SetTimeoutHeight(timeoutHeight)
	txBuilder.SetSignatures(signatures...)

	// simulate to estimate gas
	bz, err := chainClient.ClientContext().TxConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return nil, fmt.Errorf("encode tx err: %w", err)
	}

	simulateRes, err := chainClient.SimulateSignedTx(bz)
	if err != nil {
		return nil, fmt.Errorf("encode tx err: %w", err)
	}

	gasPrice := sdkmath.NewIntFromUint64(500000000)
	estimateGas := simulateRes.GasInfo.GasUsed * 2
	fee := sdkmath.NewIntFromUint64(estimateGas).Mul(gasPrice)
	txBuilder.SetGasLimit(estimateGas)
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin("lux", fee)))

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

func GetOrLinkSvmAccount(
	chainClient chainclient.ChainClient,
	clientCtx client.Context,
	cosmosPrivKey *ethsecp256k1.PrivKey,
	svmPrivKey *ed25519.PrivKey,
	luxAmount int64,
) (linkedAccount solana.PublicKey, linkTxRes *txtypes.BroadcastTxResponse, err error) {
	userPubKey := cosmosPrivKey.PubKey()
	userAddr := sdk.AccAddress(userPubKey.Address().Bytes())
	isSvmLinked, svmPubkey, err := chainClient.GetSVMAccountLink(context.Background(), userAddr)
	if err != nil {
		return solana.PublicKey{}, nil, err
	}

	if isSvmLinked {
		return svmPubkey, nil, nil
	}

	// init link msg
	svmSig, err := svmPrivKey.Sign(userAddr.Bytes())
	if err != nil {
		return solana.PublicKey{}, nil, fmt.Errorf("svm private key sign err: %w", err)
	}

	msg := &svmtypes.MsgLinkSVMAccount{
		Sender:       userAddr.String(),
		SvmPubkey:    svmPrivKey.PubKey().Bytes(),
		SvmSignature: svmSig[:],
		Amount:       sdk.NewInt64Coin("lux", luxAmount),
	}

	signedTx, err := BuildSignedTx(chainClient, []sdk.Msg{msg}, []*ethsecp256k1.PrivKey{cosmosPrivKey})
	if err != nil {
		return solana.PublicKey{}, nil, err
	}

	txBytes, err := chainClient.ClientContext().TxConfig.TxEncoder()(signedTx)
	if err != nil {
		return solana.PublicKey{}, nil, err
	}

	linkTxRes, err = chainClient.SyncBroadcastSignedTx(txBytes)
	return solana.PublicKeyFromBytes(svmPrivKey.PubKey().Bytes()), linkTxRes, err
}

func InitializePythOracle(
	chainClient chainclient.ChainClient,
	clientCtx client.Context,
	feePayerCosmosPrivHex string,
	oracleCosmosPrivHex string,
	oracleSvmPrivKey *ed25519.PrivKey,
	price int64, expo int32, conf uint64,
) (oracleSvmPubkey solana.PublicKey, err error) {
	/// initialize btc oracle
	oracleSvmPubkey = solana.PublicKeyFromBytes(oracleSvmPrivKey.PubKey().Bytes())
	feePayerCosmosPrivKey := &ethsecp256k1.PrivKey{Key: ethcommon.Hex2Bytes(feePayerCosmosPrivHex)}
	feePayerCosmosAddr := sdk.AccAddress(feePayerCosmosPrivKey.PubKey().Address().Bytes())
	oracleCosmosPrivKey := &ethsecp256k1.PrivKey{Key: ethcommon.Hex2Bytes(oracleCosmosPrivHex)}
	oracleCosmosAddr := sdk.AccAddress(oracleCosmosPrivKey.PubKey().Address().Bytes())

	isLinked, feePayerSvmPubkey, err := chainClient.GetSVMAccountLink(context.Background(), feePayerCosmosAddr)
	if err != nil {
		return solana.PublicKey{}, err
	}

	if !isLinked {
		return solana.PublicKey{}, fmt.Errorf("feePayer %s is not linked to any svm account", feePayerCosmosAddr.String())
	}

	oracleSvmPubkey, _, err = GetOrLinkSvmAccount(chainClient, clientCtx, oracleCosmosPrivKey, oracleSvmPrivKey, 0)
	if err != nil {
		panic(err)
	}

	oracleSize := uint64(3312) // deduce from Price struct
	createOracleAccountIx := system.NewCreateAccountInstruction(
		svmtypes.GetRentExemptLamportAmount(oracleSize),
		oracleSize,
		pyth.ProgramID,
		feePayerSvmPubkey,
		oracleSvmPubkey,
	).Build()

	initializeOracleIx := pyth.NewInitializeInstruction(
		price, expo, conf, oracleSvmPubkey,
	).Build()

	initOracleTx, err := solana.NewTransactionBuilder().
		AddInstruction(createOracleAccountIx).
		AddInstruction(initializeOracleIx).
		Build()
	if err != nil {
		panic(err)
	}

	initOracleMsg := ToCosmosMsg([]string{
		chainClient.FromAddress().String(),
		oracleCosmosAddr.String(),
	}, 1000_000, initOracleTx)

	oracleSignedTx, err := BuildSignedTx(
		chainClient, []sdk.Msg{initOracleMsg},
		[]*ethsecp256k1.PrivKey{
			feePayerCosmosPrivKey,
			oracleCosmosPrivKey,
		},
	)
	if err != nil {
		return solana.PublicKey{}, err
	}

	oracleTxBytes, err := chainClient.ClientContext().TxConfig.TxEncoder()(oracleSignedTx)
	if err != nil {
		return solana.PublicKey{}, err
	}

	res, err := chainClient.SyncBroadcastSignedTx(oracleTxBytes)
	if err != nil {
		return solana.PublicKey{}, err
	}

	fmt.Println("tx hash:", res.TxResponse.TxHash, "err:", res.TxResponse.RawLog)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
	return oracleSvmPubkey, nil
}

func Uint16ToLeBytes(x uint16) []byte {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, x)
	return b
}

func Uint8Ptr(b uint8) *uint8 {
	return &b
}
