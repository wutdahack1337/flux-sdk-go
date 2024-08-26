package main

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	_ "embed"

	"cosmossdk.io/math"
	astromeshtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"
	svmtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/FluxNFTLabs/sdk-go/client/svm"
	"github.com/FluxNFTLabs/sdk-go/client/svm/drift"
	pyth "github.com/FluxNFTLabs/sdk-go/client/svm/drift_pyth"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ethsecp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/mr-tron/base58"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	driftPrivKey []byte
	pythPrivKey  []byte
)

func uint16ToLeBytes(x uint16) []byte {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, x)
	return b
}

func newName(s string) [32]uint8 {
	name := [32]uint8{}
	bz := []byte(s)
	copy(name[:], bz)
	return name
}

func initializeBtcOracle(
	chainClient chainclient.ChainClient,
	clientCtx client.Context,
	feePayerCosmosPrivHex string,
	oracleCosmosPrivHex string,
	price int64, expo int32, conf uint64,
) (oraclePubkey solana.PublicKey) {
	/// initialize btc oracle
	btcOraclePrivKey := ed25519.GenPrivKeyFromSecret([]byte("btc_oracle"))
	btcOraclePubkey := solana.PublicKeyFromBytes(btcOraclePrivKey.PubKey().Bytes())
	feePayerCosmosPrivKey := &ethsecp256k1.PrivKey{Key: ethcommon.Hex2Bytes(feePayerCosmosPrivHex)}
	feePayerCosmosAddr := sdk.AccAddress(feePayerCosmosPrivKey.PubKey().Address().Bytes())
	oracleCosmosPrivKey := &ethsecp256k1.PrivKey{Key: ethcommon.Hex2Bytes(oracleCosmosPrivHex)}
	oracleCosmosAddr := sdk.AccAddress(oracleCosmosPrivKey.PubKey().Address().Bytes())

	isLinked, feePayerSvmPubkey, err := chainClient.GetSVMAccountLink(context.Background(), feePayerCosmosAddr)
	if err != nil {
		panic(err)
	}

	if !isLinked {
		panic(fmt.Errorf("feePayer %s is not linked to any svm account", feePayerCosmosAddr.String()))
	}

	fmt.Println("initialzing pyth BTC oracle account:", btcOraclePubkey.String())
	_, err = svm.LinkAccount(chainClient, clientCtx, oracleCosmosPrivKey, btcOraclePrivKey, 0)
	if err != nil {
		panic(err)
	}

	oracleSize := uint64(3840) // deduce from Price struct
	createOracleAccountIx := system.NewCreateAccountInstruction(
		svmtypes.GetRentExemptLamportAmount(oracleSize),
		oracleSize,
		pyth.ProgramID,
		feePayerSvmPubkey,
		btcOraclePubkey,
	).Build()

	initializeOracleIx := pyth.NewInitializeInstruction(
		price, expo, conf, btcOraclePubkey,
	).Build()

	initOracleTx, err := solana.NewTransactionBuilder().
		AddInstruction(createOracleAccountIx).
		AddInstruction(initializeOracleIx).
		Build()
	if err != nil {
		panic(err)
	}

	initOracleMsg := svm.ToCosmosMsg([]string{
		chainClient.FromAddress().String(),
		oracleCosmosAddr.String(),
	}, 1000_000, initOracleTx)

	oracleSignedTx, err := svm.BuildSignedTx(
		chainClient, []sdk.Msg{initOracleMsg},
		[]*ethsecp256k1.PrivKey{
			feePayerCosmosPrivKey,
			oracleCosmosPrivKey,
		},
	)
	if err != nil {
		panic(err)
	}

	oracleTxBytes, err := chainClient.ClientContext().TxConfig.TxEncoder()(oracleSignedTx)
	if err != nil {
		panic(err)
	}

	res, err := chainClient.SyncBroadcastSignedTx(oracleTxBytes)
	if err != nil {
		panic(err)
	}
	fmt.Println("tx hash:", res.TxResponse.TxHash, "err:", res.TxResponse.RawLog)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
	return btcOraclePubkey
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
	clientCtx, senderAddress, err := chaintypes.NewClientContext(
		network.ChainId,
		"user1",
		kr,
	)
	if err != nil {
		panic(err)
	}
	clientCtx = clientCtx.WithGRPCClient(cc)
	// load artifacts
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	driftPrivKey, err = os.ReadFile(dir + "/examples/chain/38_DeployDriftDEX/artifacts/drift-keypair.json")
	if err != nil {
		panic(err)
	}

	pythPrivKey, err = os.ReadFile(dir + "/examples/chain/38_DeployDriftDEX/artifacts/pyth-keypair.json")
	if err != nil {
		panic(err)
	}

	// init chain client
	chainClient, err := chainclient.NewChainClient(
		clientCtx,
		common.OptionGasPrices("500000000lux"),
	)
	if err != nil {
		panic(err)
	}

	isSvmLinked, svmPubkey, err := chainClient.GetSVMAccountLink(context.Background(), senderAddress)
	if err != nil {
		panic(err)
	}

	if !isSvmLinked {
		svmKey := ed25519.GenPrivKey() // Good practice: Backup this private key
		res, err := chainClient.LinkSVMAccount(svmKey, math.NewIntFromUint64(1_000_000_000_000_000))
		if err != nil {
			panic(err)
		}
		fmt.Println("linked sender to svm address:", base58.Encode(svmKey.PubKey().Bytes()), "txHash:", res.TxResponse.TxHash)
		svmPubkey = solana.PublicKey(svmKey.PubKey().Bytes())
	} else {
		fmt.Println("sender is already linked to svm address:", svmPubkey.String())
	}

	fmt.Println("transfer coins to create svm denom")
	coins := sdk.NewCoins(
		sdk.NewInt64Coin("btc", 10000000000),
		sdk.NewInt64Coin("usdt", 10000000000),
	)
	for _, c := range coins {
		txResp, err := chainClient.SyncBroadcastMsg(&astromeshtypes.MsgAstroTransfer{
			Sender:   senderAddress.String(),
			Receiver: senderAddress.String(),
			SrcPlane: astromeshtypes.Plane_COSMOS,
			DstPlane: astromeshtypes.Plane_SVM,
			Coin: sdk.Coin{
				Denom:  c.Denom,
				Amount: math.NewIntFromUint64(100000000000),
			},
		})
		if err != nil {
			panic(err)
		}
		fmt.Printf("=== transfer %s %s to svm ===\n", c.Amount.String(), c.Denom)
		fmt.Println("resp:", txResp.TxResponse.TxHash)
		fmt.Println("gas used/want:", txResp.TxResponse.GasUsed, "/", txResp.TxResponse.GasWanted)
	}

	denomHexMap := map[string]string{}
	for _, c := range coins {
		denomLink, err := chainClient.GetDenomLink(context.Background(), astromeshtypes.Plane_COSMOS, c.Denom, astromeshtypes.Plane_SVM)
		if err != nil {
			panic(err)
		}

		denomHexMap[c.Denom] = denomLink.DstAddr
	}

	// load program, coins id
	var pythPrivKeyBz []byte
	if err := json.Unmarshal(pythPrivKey, &pythPrivKeyBz); err != nil {
		panic(err)
	}
	pythPrivKey := &ed25519.PrivKey{Key: pythPrivKeyBz}
	pythProgramId := solana.PublicKeyFromBytes(pythPrivKey.PubKey().Bytes())
	pyth.SetProgramID(pythProgramId)

	var driftPrivKeyBz []byte
	if err := json.Unmarshal(driftPrivKey, &driftPrivKeyBz); err != nil {
		panic(err)
	}
	driftPrivKey := &ed25519.PrivKey{Key: driftPrivKeyBz}
	driftProgramId := solana.PublicKeyFromBytes(driftPrivKey.PubKey().Bytes())
	drift.SetProgramID(driftProgramId)

	usdtMintHex := denomHexMap["usdt"]
	usdtMintBz, _ := hex.DecodeString(usdtMintHex)
	usdtMint := solana.PublicKeyFromBytes(usdtMintBz)

	btcMintHex := denomHexMap["btc"]
	btcMintBz, _ := hex.DecodeString(btcMintHex)
	btcMint := solana.PublicKeyFromBytes(btcMintBz)

	fmt.Println("=== initialize BTC oracle ===")
	initializeBtcOracle(
		chainClient, clientCtx,
		"88cbead91aee890d27bf06e003ade3d4e952427e88f88d31d61d3ef5e5d54305",
		"6bf7877e9bf7590d94b57d409b0fcf4cc80f9cd427bc212b1a2dd7ff6b6802e1",
		65_000_000_000, 6, 1,
	)

	fmt.Println("=== initialize btc, usdt market states ===")
	state, _, err := solana.FindProgramAddress([][]byte{
		[]byte("drift_state"),
	}, driftProgramId)
	driftSigner, _, err := solana.FindProgramAddress([][]byte{
		[]byte("drift_signer"),
	}, driftProgramId)

	initializeIx := drift.NewInitializeInstruction(
		svmPubkey, state, usdtMint, driftSigner,
		solana.PublicKey(svmtypes.SysVarRent),
		svmtypes.SystemProgramId,
		svmtypes.SplToken2022ProgramId,
	).Build()

	spotMarketUsdt, _, err := solana.FindProgramAddress([][]byte{
		[]byte("spot_market"),
		uint16ToLeBytes(0),
	}, driftProgramId)
	if err != nil {
		panic(err)
	}

	spotMarketUsdtVault, _, err := solana.FindProgramAddress([][]byte{
		[]byte("spot_market_vault"),
		uint16ToLeBytes(0),
	}, driftProgramId)
	if err != nil {
		panic(err)
	}

	insuranceFundUsdtVault, _, err := solana.FindProgramAddress([][]byte{
		[]byte("insurance_fund_vault"),
		uint16ToLeBytes(0),
	}, driftProgramId)
	if err != nil {
		panic(err)
	}

	spotMarketBtc, _, err := solana.FindProgramAddress([][]byte{
		[]byte("spot_market"),
		uint16ToLeBytes(1),
	}, driftProgramId)
	if err != nil {
		panic(err)
	}

	// Generate PDA for spot_market_vault
	spotMarketBtcVault, _, err := solana.FindProgramAddress([][]byte{
		[]byte("spot_market_vault"),
		uint16ToLeBytes(1),
	}, driftProgramId)
	if err != nil {
		panic(err)
	}

	// Generate PDA for insurance_fund_vault
	insuranceFundBtcVault, _, err := solana.FindProgramAddress([][]byte{
		[]byte("insurance_fund_vault"),
		uint16ToLeBytes(1),
	}, driftProgramId)
	if err != nil {
		panic(err)
	}

	oracleUsdt := svmtypes.SystemProgramId // default pubkey
	oracleBtc := solana.MustPublicKeyFromBase58("3HRnxmtHQrHkooPdFZn5ZQbPTKGvBSyoTi4VVkkoT6u6")

	// Define other necessary public keys
	admin := svmPubkey
	rent := solana.SysVarRentPubkey
	systemProgram := solana.SystemProgramID
	tokenProgram := svmtypes.SplToken2022ProgramId

	optimalUtilization := uint32(8000)
	optimalBorrowRate := uint32(500)
	maxBorrowRate := uint32(1000)
	initialAssetWeight := uint32(10000)
	maintenanceAssetWeight := uint32(10000)
	initialLiabilityWeight := uint32(10000)
	maintenanceLiabilityWeight := uint32(10000)
	imfFactor := uint32(0)
	liquidatorFee := uint32(50)
	ifLiquidationFee := uint32(25)
	activeStatus := true
	assetTier := drift.AssetTierIsolated // TODO: Inspect what is this tier
	scaleInitialAssetWeightStart := uint64(1000000000)
	withdrawGuardThreshold := uint64(500000000)
	orderTickSize := uint64(1000)
	orderStepSize := uint64(100)
	ifTotalFactor := uint32(10)
	nameUsdt := newName("market_usdt")
	nameBtc := newName("market_btc")

	initializeQuoteSpotMarketIx := drift.NewInitializeSpotMarketInstruction(
		/* Parameters */
		optimalUtilization, optimalBorrowRate, maxBorrowRate,
		drift.OracleSourceQuoteAsset,
		initialAssetWeight, maintenanceAssetWeight,
		initialLiabilityWeight, maintenanceLiabilityWeight, imfFactor,
		liquidatorFee, ifLiquidationFee, activeStatus, assetTier,
		scaleInitialAssetWeightStart, withdrawGuardThreshold,
		orderTickSize, orderStepSize, ifTotalFactor, nameUsdt,
		/* Accounts */
		spotMarketUsdt, usdtMint, spotMarketUsdtVault,
		insuranceFundUsdtVault, driftSigner, state,
		oracleUsdt, admin, rent, systemProgram, tokenProgram,
	).Build()

	initializeBtcSpotMarketIx := drift.NewInitializeSpotMarketInstruction(
		/* Parameters */
		optimalUtilization, optimalBorrowRate, maxBorrowRate,
		drift.OracleSourcePyth,
		8000, 9000,
		12000, 11000, 105000,
		liquidatorFee, ifLiquidationFee, activeStatus, assetTier,
		scaleInitialAssetWeightStart, withdrawGuardThreshold,
		orderTickSize, orderStepSize, ifTotalFactor, nameBtc,
		/* Accounts */
		spotMarketBtc, btcMint, spotMarketBtcVault,
		insuranceFundBtcVault, driftSigner, state,
		oracleBtc, admin, rent, systemProgram, tokenProgram,
	).Build()

	initializeTx, err := solana.NewTransactionBuilder().
		AddInstruction(initializeIx).
		AddInstruction(initializeQuoteSpotMarketIx).
		AddInstruction(initializeBtcSpotMarketIx).Build()
	if err != nil {
		panic(err)
	}

	marketExist := true
	_, err = chainClient.GetSvmAccount(context.Background(), spotMarketBtc.String())
	if err != nil && !strings.Contains(err.Error(), "not existed") {
		panic(err)
	}

	if err != nil {
		marketExist = false
	}

	if !marketExist {
		svmMsg := svm.ToCosmosMsg([]string{senderAddress.String()}, 1000_000, initializeTx)
		res, err := chainClient.SyncBroadcastMsg(svmMsg)
		if err != nil {
			panic(err)
		}

		fmt.Println("== init account and create market ==")
		fmt.Println("tx hash:", res.TxResponse.TxHash)
		fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
	} else {
		fmt.Println("account and market already initialized")
	}
}
