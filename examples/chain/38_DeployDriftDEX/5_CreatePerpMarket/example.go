package main

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strconv"
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
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ethsecp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/mr-tron/base58"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	driftPrivKey []byte
	pythPrivKey  []byte

	btcOraclePrivKey = ed25519.GenPrivKeyFromSecret([]byte("btc_oracle"))
	btcOraclePubkey  = solana.PublicKeyFromBytes(btcOraclePrivKey.PubKey().Bytes())
	ethOraclePrivKey = ed25519.GenPrivKeyFromSecret([]byte("eth_oracle"))
	ethOraclePubkey  = solana.PublicKeyFromBytes(btcOraclePrivKey.PubKey().Bytes())
	solOraclePrivKey = ed25519.GenPrivKeyFromSecret([]byte("sol_oracle"))
	solOraclePubkey  = solana.PublicKeyFromBytes(btcOraclePrivKey.PubKey().Bytes())
)

type Market struct {
	Name                string
	InitialOraclePrice  uint64
	OracleSvmPrivKey    *ed25519.PrivKey
	OracleCosmosPrivKey *ethsecp256k1.PrivKey
}

func newName(s string) [32]uint8 {
	name := [32]uint8{}
	bz := []byte(s)
	copy(name[:], bz)
	return name
}

func mustParseHex(hexString string) []byte {
	bz, err := hex.DecodeString(hexString)
	if err != nil {
		panic(err)
	}

	return bz
}

func newUint128(s string) bin.Uint128 {
	// Parse the string as a big.Int
	bigInt := new(big.Int)
	_, success := bigInt.SetString(s, 10) // Assuming the string is in base 10
	if !success {
		panic(fmt.Errorf("invalid string for Uint128 conversion: %s", s))
	}

	lo := bigInt.Uint64() // Lower 64 bits
	// Shift right by 64 bits to get the upper 64 bits
	hi := new(big.Int).Rsh(bigInt, 64).Uint64()

	// Create and return the Uint128
	return bin.Uint128{
		Lo:         lo,
		Hi:         hi,
		Endianness: binary.BigEndian,
	}

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

	fmt.Println("=== transfer coins to svm ===")
	coins := sdk.NewCoins(
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

	usdtMintHex := denomHexMap["usdt"]
	usdtMintBz, _ := hex.DecodeString(usdtMintHex)
	usdtMint := solana.PublicKeyFromBytes(usdtMintBz)

	markets := []Market{
		{
			Name:               "btc",
			InitialOraclePrice: 65000_000_000,
			OracleSvmPrivKey:   btcOraclePrivKey,
			OracleCosmosPrivKey: &ethsecp256k1.PrivKey{
				Key: mustParseHex("6bf7877e9bf7590d94b57d409b0fcf4cc80f9cd427bc212b1a2dd7ff6b6802e1"), // note: do not use this key on mainnet
			},
		},
		{
			Name:               "eth",
			InitialOraclePrice: 3000_000_000,
			OracleSvmPrivKey:   ethOraclePrivKey,
			OracleCosmosPrivKey: &ethsecp256k1.PrivKey{
				Key: mustParseHex("6bf7877e9bf7590d94b57d409b0fcf4cc80f9cd427bc212b1a2dd7ff6b6802e2"), // note: do not use this key on mainnet
			},
		},
		{
			Name:               "sol",
			InitialOraclePrice: 150_000_000,
			OracleSvmPrivKey:   solOraclePrivKey,
			OracleCosmosPrivKey: &ethsecp256k1.PrivKey{
				Key: mustParseHex("6bf7877e9bf7590d94b57d409b0fcf4cc80f9cd427bc212b1a2dd7ff6b6802e3"), // note: do not use this key on mainnet
			},
		},
	}

	fmt.Println("=== initialize oracles ===")
	msgSends := []sdk.Msg{}
	for _, m := range markets {
		oracleCosmosAddr := sdk.AccAddress(m.OracleCosmosPrivKey.PubKey().Address().Bytes())
		msgSends = append(msgSends, &banktypes.MsgSend{
			FromAddress: senderAddress.String(),
			ToAddress:   oracleCosmosAddr.String(),
			Amount:      sdk.NewCoins(sdk.NewInt64Coin("lux", 1_000_000_000_000_000_000)),
		})
	}
	res, err := chainClient.SyncBroadcastMsg(msgSends...)
	if err != nil {
		panic(err)
	}
	fmt.Println("create oracle cosmos accounts")
	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)

	for _, m := range markets {
		fmt.Println("init oracle of perp market", m.Name, "oracle account:", solana.PublicKeyFromBytes(m.OracleSvmPrivKey.PubKey().Bytes()).String())
		svm.InitializePythOracle(
			chainClient, clientCtx,
			"88cbead91aee890d27bf06e003ade3d4e952427e88f88d31d61d3ef5e5d54305",
			hex.EncodeToString(m.OracleCosmosPrivKey.Key),
			m.OracleSvmPrivKey,
			int64(m.InitialOraclePrice), 6, 1,
		)
	}

	fmt.Println("=== initialize perp market states ===")
	initializeMarketTxBuilder := solana.NewTransactionBuilder()

	driftSigner, _, err := solana.FindProgramAddress([][]byte{
		[]byte("drift_signer"),
	}, driftProgramId)

	spotMarketUsdt, _, err := solana.FindProgramAddress([][]byte{
		[]byte("spot_market"),
		svm.Uint16ToLeBytes(0),
	}, driftProgramId)
	if err != nil {
		panic(err)
	}

	state, _, err := solana.FindProgramAddress([][]byte{
		[]byte("drift_state"),
	}, driftProgramId)

	driftStateExist := true
	_, err = chainClient.GetSvmAccount(context.Background(), state.String())
	if err != nil && !strings.Contains(err.Error(), "not existed") {
		panic(err)
	}
	if err != nil {
		driftStateExist = false
	}

	if !driftStateExist {
		driftSigner, _, err := solana.FindProgramAddress([][]byte{
			[]byte("drift_signer"),
		}, driftProgramId)
		if err != nil {
			panic(err)
		}

		initializeStateIx := drift.NewInitializeInstruction(
			svmPubkey, state, usdtMint, driftSigner,
			solana.PublicKey(svmtypes.SysVarRent),
			svmtypes.SystemProgramId,
			svmtypes.SplToken2022ProgramId,
		).Build()

		initializeMarketTxBuilder = initializeMarketTxBuilder.AddInstruction(initializeStateIx)
	}

	// create quote (usdt) market if it doesn't exist
	quoteMarketExists := true
	_, err = chainClient.GetSvmAccount(context.Background(), spotMarketUsdt.String())
	if err != nil && !strings.Contains(err.Error(), "not existed") {
		panic(err)
	}
	if err != nil {
		quoteMarketExists = false
	}

	if !quoteMarketExists {
		spotMarketUsdtVault, _, err := solana.FindProgramAddress([][]byte{
			[]byte("spot_market_vault"),
			svm.Uint16ToLeBytes(0),
		}, driftProgramId)
		if err != nil {
			panic(err)
		}

		insuranceFundUsdtVault, _, err := solana.FindProgramAddress([][]byte{
			[]byte("insurance_fund_vault"),
			svm.Uint16ToLeBytes(0),
		}, driftProgramId)
		if err != nil {
			panic(err)
		}

		oracleUsdt := solana.PublicKey{} // empty for quote asset
		admin := svmPubkey
		rent := solana.SysVarRentPubkey
		systemProgram := solana.SystemProgramID
		tokenProgram := svmtypes.SplToken2022ProgramId
		optimalUtilization := uint32(8000)
		optimalBorrowRate := uint32(500)
		maxBorrowRate := uint32(1000)
		liquidatorFee := uint32(50)
		ifLiquidationFee := uint32(25)
		activeStatus := true
		assetTier := drift.AssetTierIsolated
		scaleInitialAssetWeightStart := uint64(1000000000)
		withdrawGuardThreshold := uint64(500000000)
		orderTickSize := uint64(1000)
		orderStepSize := uint64(100)
		ifTotalFactor := uint32(10)

		initializeQuoteSpotMarketIx := drift.NewInitializeSpotMarketInstruction(
			optimalUtilization, optimalBorrowRate, maxBorrowRate,
			drift.OracleSourceQuoteAsset,
			10000, 10000,
			10000, 10000, 0,
			liquidatorFee,
			ifLiquidationFee, activeStatus,
			assetTier, scaleInitialAssetWeightStart,
			withdrawGuardThreshold,
			orderTickSize, orderStepSize,
			ifTotalFactor,
			newName("usdt"),
			spotMarketUsdt, usdtMint, spotMarketUsdtVault,
			insuranceFundUsdtVault, driftSigner, state,
			oracleUsdt, admin, rent, systemProgram, tokenProgram,
		).Build()
		initializeMarketTxBuilder = initializeMarketTxBuilder.AddInstruction(initializeQuoteSpotMarketIx)
	}

	// create three perps markets
	for idx, m := range markets {
		marketIndex := uint16(idx)
		perpMarket, _, err := solana.FindProgramAddress([][]byte{
			[]byte("perp_market"),
			svm.Uint16ToLeBytes(marketIndex),
		}, driftProgramId)
		if err != nil {
			panic(err)
		}

		oracle := solana.PublicKeyFromBytes(m.OracleSvmPrivKey.PubKey().Bytes())

		// Define other necessary public keys
		admin := svmPubkey
		rent := solana.SysVarRentPubkey
		systemProgram := solana.SystemProgramID
		ammBaseAssetReserve := newUint128("1000000000")
		ammQuoteAssetReserve := newUint128("1000000000")
		ammPeriodicity := int64(3600)
		ammPegMultiplier := newUint128(strconv.FormatUint(m.InitialOraclePrice, 10))
		oracleSource := drift.OracleSourcePyth
		contractTier := drift.ContractTierA
		marginRatioInitial := uint32(10000)
		marginRatioMaintenance := uint32(5000)
		liquidatorFee := uint32(50)
		ifLiquidationFee := uint32(25)
		imfFactor := uint32(100)
		activeStatus := true
		baseSpread := uint32(10)
		maxSpread := uint32(100)
		maxOpenInterest := newUint128("1000000000")
		maxRevenueWithdrawPerPeriod := uint64(500000000)
		quoteMaxInsurance := uint64(100000000)
		orderStepSize := uint64(100)
		orderTickSize := uint64(10)
		minOrderSize := uint64(1)
		concentrationCoefScale := newUint128("10")
		curveUpdateIntensity := uint8(1)
		ammJitIntensity := uint8(1)
		name := newName(m.Name)

		// Build the instruction
		initializePerpMarketIx := drift.NewInitializePerpMarketInstruction(
			// Parameters
			marketIndex, ammBaseAssetReserve, ammQuoteAssetReserve,
			ammPeriodicity, ammPegMultiplier, oracleSource, contractTier,
			marginRatioInitial, marginRatioMaintenance, liquidatorFee,
			ifLiquidationFee, imfFactor, activeStatus, baseSpread, maxSpread,
			maxOpenInterest, maxRevenueWithdrawPerPeriod, quoteMaxInsurance,
			orderStepSize, orderTickSize, minOrderSize, concentrationCoefScale,
			curveUpdateIntensity, ammJitIntensity, name,
			// Accounts
			admin, state, perpMarket, oracle, rent, systemProgram,
		).Build()

		initializeMarketTxBuilder = initializeMarketTxBuilder.AddInstruction(initializePerpMarketIx)
		fmt.Printf("perp market %s account: %s\n", m.Name, perpMarket.String())
	}

	initializeMarketTx, err := initializeMarketTxBuilder.Build()
	if err != nil {
		panic(err)
	}

	svmMsg := svmtypes.ToCosmosMsg([]string{senderAddress.String()}, 1000_000, initializeMarketTx)
	res, err = chainClient.SyncBroadcastMsg(svmMsg)
	if err != nil {
		panic(err)
	}

	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
}
