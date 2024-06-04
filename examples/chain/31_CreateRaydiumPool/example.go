package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"cosmossdk.io/math"
	astromeshtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"
	svmtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/FluxNFTLabs/sdk-go/client/svm"
	raydium_cp_swap "github.com/FluxNFTLabs/sdk-go/examples/chain/31_CreateRaydiumPool/raydium"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const MaxComputeBudget = 10000000

const InitialBtcUsdtPrice = uint64(70000)

var TokenProgramID = solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")

func init() {
	// use token2022 program by default
	solana.TokenProgramID = svmtypes.SplToken2022ProgramId
}

func calculateLpToken(
	maxToken0Amount, maxToken1Amount uint64,
	token0Vault, token1Vault uint64,
	currentLpAmount uint64,
) (lpAmount, actualToken0Deposit, actualToken1Deposit uint64) {
	// lpAmount / currentLpAmount * token0Vault <= token0Amount
	// lpAmount / currentLpAmount * token1Vault <= token1Amount
	lpAmountToken0Based := math.NewIntFromUint64(maxToken0Amount).Mul(math.NewIntFromUint64(currentLpAmount)).Quo(math.NewIntFromUint64(token0Vault))
	lpAmountToken1Based := math.NewIntFromUint64(maxToken1Amount).Mul(math.NewIntFromUint64(currentLpAmount)).Quo(math.NewIntFromUint64(token1Vault))

	netLpAmount := lpAmountToken0Based
	if netLpAmount.GT(lpAmountToken1Based) {
		lpAmountToken1Based = netLpAmount
	}

	lpAmount = netLpAmount.Uint64()
	actualToken0Deposit = netLpAmount.Mul(math.NewIntFromUint64(token0Vault)).Quo(math.NewIntFromUint64(currentLpAmount)).Uint64()
	actualToken1Deposit = netLpAmount.Mul(math.NewIntFromUint64(token1Vault)).Quo(math.NewIntFromUint64(currentLpAmount)).Uint64()
	return
}

func parseResult(txResp *txtypes.BroadcastTxResponse) (res *astromeshtypes.MsgAstroTransferResponse, err error) {
	hexResp, err := hex.DecodeString(txResp.TxResponse.Data)
	if err != nil {
		panic(err)
	}

	// decode result to get contract address
	var txData sdk.TxMsgData
	if err := txData.Unmarshal(hexResp); err != nil {
		panic(err)
	}

	var r astromeshtypes.MsgAstroTransferResponse
	if err := r.Unmarshal(txData.MsgResponses[0].Value); err != nil {
		return nil, err
	}
	return &r, nil
}

func createSvmAccountIfNotExist(chainClient chainclient.ChainClient, ctx context.Context, senderAddress sdk.Address, svmAccount solana.PublicKey) {
	senderSvmAccount := solana.PublicKey(ethcrypto.Keccak256(senderAddress.Bytes()))
	_, err := chainClient.GetSvmAccount(ctx, svmAccount.String())
	msgs := []sdk.Msg{}
	if err != nil {
		if strings.Contains(err.Error(), "Account not existed") {
			fmt.Printf("svm account %s not exist, going to create account\n", svmAccount.String())
			createAccountIx := system.NewCreateAccountInstruction(0, 0, system.ProgramID, senderSvmAccount, svmAccount)
			svmTx, err := solana.NewTransactionBuilder().AddInstruction(createAccountIx.Build()).Build()
			if err != nil {
				// never reach this but added to be safe
				panic(err)
			}
			msgs = append(msgs, svm.ToCosmosMsg(senderAddress.String(), 1000000, svmTx))
		} else {
			panic(err)
		}
	}

	if len(msgs) > 0 {
		res, err := chainClient.SyncBroadcastMsg(msgs...)
		if err != nil {
			panic(err)
		}
		fmt.Println("----- log: action: Create Svm account ------")
		fmt.Println("cosmos account:", senderAddress.String())
		fmt.Println("svm account:", solana.PublicKey(senderSvmAccount).String())
		fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
	}

	fmt.Println("svm account already created:", solana.PublicKey(senderSvmAccount).String())
}

func transfer(chainClient chainclient.ChainClient, ctx context.Context, senderAddress sdk.Address, denom string, amount int64) {
	senderSvmAccount := solana.PublicKey(ethcrypto.Keccak256(senderAddress.Bytes()))
	res, err := chainClient.SyncBroadcastMsg(&astromeshtypes.MsgAstroTransfer{
		Sender:   senderAddress.String(),
		Receiver: senderAddress.String(),
		SrcPlane: astromeshtypes.Plane_COSMOS,
		DstPlane: astromeshtypes.Plane_SVM,
		Coin:     sdk.NewCoin(denom, math.NewInt(amount)),
	})
	if err != nil {
		panic(err)
	}

	parsedResult, err := parseResult(res)
	if err != nil {
		panic(err)
	}

	ataAccount, _, err := solana.FindAssociatedTokenAddress(solana.PublicKey(senderSvmAccount), solana.PublicKey(parsedResult.DestinationDenom))
	if err != nil {
		panic(err)
	}
	fmt.Println("----- log: action: transfer ------")
	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
	fmt.Println("cosmos account:", senderAddress.String())
	fmt.Println("svm account:", solana.PublicKey(senderSvmAccount).String())
	fmt.Printf("%s mint svm: %s\n", denom, solana.PublicKey(parsedResult.DestinationDenom).String())
	fmt.Println("ata account (token22):", ataAccount.String())
}

func transferBalance(chainClient chainclient.ChainClient, ctx context.Context, senderAddress sdk.Address) {
	transfer(chainClient, ctx, senderAddress, "btc", 2_00000000)
	transfer(chainClient, ctx, senderAddress, "usdt", 160000_000000)
}

func createAmmConfig(
	chainClient chainclient.ChainClient,
	ctx context.Context,
	senderAddress sdk.AccAddress,
	raydiumAdmin solana.PublicKey,
	raydiumSwapProgram solana.PublicKey,
) (ammConfigAccount solana.PublicKey) {
	ammConfigAccount, _, err := solana.FindProgramAddress([][]byte{
		[]byte("amm_config"),
		{0, 0},
	}, raydiumSwapProgram)
	if err != nil {
		panic(err)
	}

	_, err = chainClient.GetSvmAccount(ctx, ammConfigAccount.String())
	if err == nil {
		fmt.Println("amm config account already created:", ammConfigAccount.String())
		return ammConfigAccount
	}

	createAmmConfigIx := raydium_cp_swap.NewCreateAmmConfigInstruction(
		0,
		1000,         // trade fee (rate) / 10^6 = 0.1%
		2000,         // protocol fee
		1000,         // fund fee
		0,            // create pool fee
		raydiumAdmin, // config owner
		ammConfigAccount,
		solana.SystemProgramID,
	)

	createAmmConfigTx, err := solana.NewTransactionBuilder().AddInstruction(createAmmConfigIx.Build()).Build()
	if err != nil {
		panic(err)
	}

	res, err := chainClient.SyncBroadcastMsg(svm.ToCosmosMsg(senderAddress.String(), MaxComputeBudget, createAmmConfigTx))
	if err != nil {
		panic(err)
	}

	fmt.Println("----- log: action: create AMM Config ------")
	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
	fmt.Println("amm config account", ammConfigAccount.String())
	return ammConfigAccount
}

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

func initializeAmmPool(
	chainClient chainclient.ChainClient,
	ctx context.Context,
	senderAddress sdk.AccAddress,
	raydiumSwapProgram solana.PublicKey,
	raydiumFeeReceiver solana.PublicKey,
	ammConfigAccount solana.PublicKey,
	baseTokenMint solana.PublicKey,
	quoteTokenMint solana.PublicKey,
) (poolStateAccount solana.PublicKey, poolState *raydium_cp_swap.PoolState) {
	poolStateAccount, _, err := solana.FindProgramAddress([][]byte{
		[]byte("pool"),
		ammConfigAccount[:],
		baseTokenMint[:],
		quoteTokenMint[:],
	}, raydiumSwapProgram)
	if err != nil {
		panic(err)
	}

	poolStateResponse, err := chainClient.GetSvmAccount(ctx, poolStateAccount.String())
	if err == nil {
		fmt.Println("pool already created:", poolStateAccount.String())
		poolState := new(raydium_cp_swap.PoolState)
		if err := poolState.UnmarshalWithDecoder(bin.NewBinDecoder(poolStateResponse.Account.Data)); err != nil {
			panic(err)
		}
		return poolStateAccount, poolState
	}

	senderSvmAccount := solana.PublicKey(ethcrypto.Keccak256(senderAddress.Bytes()))
	authorityAccount, _, err := solana.FindProgramAddress([][]byte{
		[]byte("vault_and_lp_mint_auth_seed"),
	}, raydiumSwapProgram)
	if err != nil {
		panic(err)
	}

	lpMint, _, err := solana.FindProgramAddress([][]byte{
		[]byte("pool_lp_mint"),
		poolStateAccount[:],
	}, raydiumSwapProgram)

	tokens := []solana.PublicKey{baseTokenMint, quoteTokenMint}
	if bytes.Compare(baseTokenMint[:], quoteTokenMint[:]) > 0 {
		tokens = []solana.PublicKey{quoteTokenMint, baseTokenMint}
	}

	creatorToken0Ata, _, err := solana.FindAssociatedTokenAddress(solana.PublicKey(senderSvmAccount), tokens[0])
	if err != nil {
		panic(err)
	}

	creatorToken1Ata, _, err := solana.FindAssociatedTokenAddress(solana.PublicKey(senderSvmAccount), tokens[1])
	if err != nil {
		panic(err)
	}

	creatorLpAta := MustFindAta(solana.PublicKey(senderSvmAccount), TokenProgramID, lpMint, svmtypes.AssociatedTokenProgramId)
	tokens0Vault, _, err := solana.FindProgramAddress([][]byte{
		[]byte("pool_vault"),
		poolStateAccount[:],
		tokens[0][:],
	}, raydiumSwapProgram)
	if err != nil {
		panic(err)
	}

	tokens1Vault, _, err := solana.FindProgramAddress([][]byte{
		[]byte("pool_vault"),
		poolStateAccount[:],
		tokens[1][:],
	}, raydiumSwapProgram)
	if err != nil {
		panic(err)
	}

	oracleObserver, _, err := solana.FindProgramAddress([][]byte{
		[]byte("observation"),
		poolStateAccount[:],
	}, raydiumSwapProgram)
	if err != nil {
		panic(err)
	}

	btcAmount := uint64(100000000)
	usdtAmount := btcAmount * InitialBtcUsdtPrice * 1000000 / uint64(100000000)
	createPoolIx := raydium_cp_swap.NewInitializeInstruction(
		uint64(btcAmount),
		uint64(usdtAmount),
		0,
		senderSvmAccount,
		ammConfigAccount,
		authorityAccount,
		poolStateAccount,
		tokens[0],
		tokens[1],
		lpMint,
		creatorToken0Ata,
		creatorToken1Ata,
		creatorLpAta,
		tokens0Vault,
		tokens1Vault,
		raydiumFeeReceiver,
		oracleObserver,
		TokenProgramID,
		svmtypes.SplToken2022ProgramId,
		svmtypes.SplToken2022ProgramId,
		svmtypes.AssociatedTokenProgramId,
		svmtypes.SystemProgramId,
		solana.PublicKey(svmtypes.SysVarRent),
	)

	tx, err := solana.NewTransactionBuilder().AddInstruction(createPoolIx.Build()).Build()
	if err != nil {
		panic(err)
	}

	res, err := chainClient.SyncBroadcastMsg(svm.ToCosmosMsg(senderAddress.String(), MaxComputeBudget, tx))
	if err != nil {
		panic(err)
	}

	fmt.Println("----- log: action: Create pool ------")
	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
	fmt.Println("pool state account:", poolStateAccount.String())
	fmt.Println("creator owner lp amount:", mustGetTokenAccount(chainClient, ctx, creatorLpAta).Amount)

	poolStateResponse, err = chainClient.GetSvmAccount(ctx, poolStateAccount.String())
	if err != nil {
		panic(err)
	}
	poolState = new(raydium_cp_swap.PoolState)
	if err := poolState.UnmarshalWithDecoder(bin.NewBinDecoder(poolStateResponse.Account.Data)); err != nil {
		panic(err)
	}
	return poolStateAccount, poolState
}

func mustGetTokenAccount(
	chainClient chainclient.ChainClient,
	ctx context.Context,
	account solana.PublicKey,
) *token.Account {
	var tokenAcc token.Account
	acc, err := chainClient.GetSvmAccount(ctx, account.String())
	if err != nil {
		panic(err)
	}

	if err := tokenAcc.UnmarshalWithDecoder(bin.NewBinDecoder(acc.Account.Data)); err != nil {
		panic(err)
	}
	return &tokenAcc
}

func swapBaseInput(
	chainClient chainclient.ChainClient,
	ctx context.Context,
	senderAddress sdk.AccAddress,
	raydiumSwapProgram solana.PublicKey,
	authorityAccount solana.PublicKey, // TODO: What is this for?
	amountIn uint64,
	minAmountOut int64,
	inputTokenAccount solana.PublicKey,
	outputTokenAccount solana.PublicKey,
	ammConfigAccount solana.PublicKey,
	poolState solana.PublicKey,
	inputVault solana.PublicKey,
	outputVault solana.PublicKey,
	inputTokenMint solana.PublicKey,
	outputTokenMint solana.PublicKey,
	observerState solana.PublicKey,
) {
	senderSvmAccount := solana.PublicKey(ethcrypto.Keccak256(senderAddress.Bytes()))
	ix := raydium_cp_swap.NewSwapBaseInputInstruction(
		amountIn,
		uint64(minAmountOut),
		senderSvmAccount,
		authorityAccount,
		ammConfigAccount,
		poolState,
		inputTokenAccount,
		outputTokenAccount,
		inputVault,
		outputVault,
		svmtypes.SplToken2022ProgramId,
		svmtypes.SplToken2022ProgramId,
		inputTokenMint,
		outputTokenMint,
		observerState,
	)

	tx, err := solana.NewTransactionBuilder().AddInstruction(ix.Build()).Build()
	if err != nil {
		panic(err)
	}

	res, err := chainClient.SyncBroadcastMsg(svm.ToCosmosMsg(senderAddress.String(), MaxComputeBudget, tx))
	if err != nil {
		panic(err)
	}

	fmt.Println("----- log: action: Swap ------")
	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
}

func createNativeMint(
	chainClient chainclient.ChainClient,
	ctx context.Context,
	senderAddress sdk.AccAddress,
) {
	_, err := chainClient.GetSvmAccount(ctx, Sol22NativeMint.String())
	if err == nil {
		fmt.Println("sol native mint already created:", Sol22NativeMint.String())
		return
	}

	senderSvmAccount := solana.PublicKey(ethcrypto.Keccak256(senderAddress.Bytes()))
	createMintIx := NewCreateNativeMintInstruction(
		senderSvmAccount, Sol22NativeMint, solana.SystemProgramID,
	)
	tx, err := solana.NewTransactionBuilder().AddInstruction(createMintIx).Build()
	if err != nil {
		panic(err)
	}

	res, err := chainClient.SyncBroadcastMsg(svm.ToCosmosMsg(senderAddress.String(), 1000000, tx))
	if err != nil {
		panic(err)
	}

	fmt.Println("----- log: action: create native mint ------")
	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
	fmt.Println("sol native mint created:", Sol22NativeMint.String())
}

func createFeeReceiverAccount(
	chainClient chainclient.ChainClient,
	ctx context.Context,
	senderAddress sdk.AccAddress,
	owner solana.PublicKey,
) solana.PublicKey {
	senderSvmAccount := solana.PublicKey(ethcrypto.Keccak256(senderAddress.Bytes()))
	ownerSolAta, _, err := solana.FindAssociatedTokenAddress(owner, Sol22NativeMint)
	if err != nil {
		panic(err)
	}
	accData, err := chainClient.GetSvmAccount(ctx, ownerSolAta.String())
	if err == nil {
		fmt.Println("sol receiver ATA created:", ownerSolAta.String())
		var a = new(token.Account)
		err := a.UnmarshalWithDecoder(bin.NewBinDecoder(accData.Account.Data))
		if err != nil {
			panic(err)
		}
		bz, _ := json.Marshal(a)
		fmt.Println("account token22:", string(bz), "owner:", solana.PublicKeyFromBytes(accData.Account.Owner).String())
		return ownerSolAta
	}

	createAtaIx := associatedtokenaccount.NewCreateInstruction(
		senderSvmAccount,
		owner,
		Sol22NativeMint,
	).Build()

	tx, err := solana.NewTransactionBuilder().AddInstruction(createAtaIx).Build()
	if err != nil {
		panic(err)
	}

	res, err := chainClient.SyncBroadcastMsg(svm.ToCosmosMsg(senderAddress.String(), 1000000, tx))
	if err != nil {
		panic(err)
	}

	fmt.Println("----- log: action: create fee receiver ATA ------")
	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
	fmt.Println("admin SOL (token2022) ata:", ownerSolAta)
	return ownerSolAta
}

func deposit(
	chainClient chainclient.ChainClient,
	ctx context.Context,
	senderAddress sdk.AccAddress,
	maxToken0Amount, maxToken1Amount uint64,
	token0Account solana.PublicKey,
	token1Account solana.PublicKey,
	token0Vault solana.PublicKey,
	token1Vault solana.PublicKey,
	token0Mint solana.PublicKey,
	token1Mint solana.PublicKey,
	owner solana.PublicKey,
	authority solana.PublicKey, // program PDA as mint authority
	poolStateAccount solana.PublicKey,
	lpMint solana.PublicKey,
	poolState raydium_cp_swap.PoolState,
) {
	senderSvmAccount := solana.PublicKey(ethcrypto.Keccak256(senderAddress.Bytes()))
	ownerLpToken := MustFindAta(
		senderSvmAccount, TokenProgramID, lpMint, svmtypes.AssociatedTokenProgramId,
	)

	// get token 0 vault balance
	token0VaultBalance := mustGetTokenAccount(chainClient, ctx, token0Vault)
	// get token 1 vault balance
	token1VaultBalance := mustGetTokenAccount(chainClient, ctx, token1Vault)

	lpAmount, used0, used1 := calculateLpToken(
		maxToken0Amount, maxToken1Amount,
		token0VaultBalance.Amount-(poolState.ProtocolFeesToken0+poolState.FundFeesToken0),
		token1VaultBalance.Amount-(poolState.ProtocolFeesToken1+poolState.FundFeesToken1),
		poolState.LpSupply,
	)
	fmt.Println("real lp amount:", lpAmount, used0, used1)
	depositIx := raydium_cp_swap.NewDepositInstruction(
		lpAmount,
		maxToken0Amount,
		maxToken1Amount,
		senderSvmAccount,
		authority,
		poolStateAccount,
		ownerLpToken,
		token0Account,
		token1Account,
		token0Vault,
		token1Vault,
		TokenProgramID,
		svmtypes.SplToken2022ProgramId,
		token0Mint,
		token1Mint,
		lpMint,
	).Build()

	tx, err := solana.NewTransactionBuilder().AddInstruction(depositIx).Build()
	if err != nil {
		panic(err)
	}

	res, err := chainClient.SyncBroadcastMsg(svm.ToCosmosMsg(senderAddress.String(), 1000000, tx))
	if err != nil {
		panic(err)
	}

	fmt.Println("----- log: action: deposit ------")
	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
	fmt.Println("input vault balance after deposit:", mustGetTokenAccount(chainClient, ctx, token0Vault).Amount)
	fmt.Println("output vault balance after deposit:", mustGetTokenAccount(chainClient, ctx, token1Vault).Amount)
	fmt.Println("new owner lp amount:", mustGetTokenAccount(chainClient, ctx, ownerLpToken).Amount)
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
	cc, err := grpc.Dial("localhost:9900", grpc.WithTransportCredentials(insecure.NewCredentials()))
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

	// init chain client
	chainClient, err := chainclient.NewChainClient(
		clientCtx,
		common.OptionGasPrices("500000000lux"),
	)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	senderSvmAddress := solana.PublicKey(ethcrypto.Keccak256(senderAddress))
	createSvmAccountIfNotExist(chainClient, ctx, senderAddress, senderSvmAddress)

	// get btc, usdt denom on svm
	var btcMint, usdtMint solana.PublicKey
	btcLink, err := chainClient.GetDenomLink(ctx, astromeshtypes.Plane_COSMOS, "btc", astromeshtypes.Plane_SVM)
	if err != nil {
		if strings.Contains(err.Error(), "doesn't exist") {
			// start transfer to get btc, usdt mint
			transferBalance(chainClient, ctx, senderAddress)
			btcLink, err = chainClient.GetDenomLink(ctx, astromeshtypes.Plane_COSMOS, "btc", astromeshtypes.Plane_SVM)
		} else {
			panic(err)
		}
	}

	if err != nil {
		panic(err)
	}

	denomBytes, _ := hex.DecodeString(btcLink.DstAddr)
	btcMint = solana.PublicKey(denomBytes)

	usdtLink, err := chainClient.GetDenomLink(ctx, astromeshtypes.Plane_COSMOS, "usdt", astromeshtypes.Plane_SVM)
	if err != nil {
		panic(err)
	}

	denomBytes, _ = hex.DecodeString(usdtLink.DstAddr)
	usdtMint = solana.PublicKey(denomBytes)

	fmt.Println("btc mint:", btcMint.String())
	fmt.Println("usdt mint:", usdtMint.String())
	adminAccount := solana.MustPublicKeyFromBase58("GThUX1Atko4tqhN2NaiTazWSeFWMuiUvfFnyJyUghFMJ")
	createNativeMint(chainClient, ctx, senderAddress)
	feeReceiverAccount := createFeeReceiverAccount(chainClient, ctx, senderAddress, adminAccount)

	// create amm config
	raydiumProgramId := solana.MustPublicKeyFromBase58("CPMMoo8L3F4NbTegBCKVNunggL7H1ZpdTHKxQB5qKP1C")
	raydium_cp_swap.SetProgramID(raydiumProgramId)
	ammConfigAccount := createAmmConfig(chainClient, ctx, senderAddress, adminAccount, raydiumProgramId)

	// create pool
	authorityAccount, _, err := solana.FindProgramAddress([][]byte{
		[]byte("vault_and_lp_mint_auth_seed"),
	}, raydiumProgramId)
	if err != nil {
		panic(err)
	}

	poolAccount, poolState := initializeAmmPool(
		chainClient,
		ctx, senderAddress,
		raydiumProgramId,
		feeReceiverAccount,
		ammConfigAccount,
		btcMint, usdtMint,
	)

	traderBtcAta := MustFindAta(senderSvmAddress, svmtypes.SplToken2022ProgramId, btcMint, svmtypes.AssociatedTokenProgramId)
	traderUsdtAta := MustFindAta(senderSvmAddress, svmtypes.SplToken2022ProgramId, usdtMint, svmtypes.AssociatedTokenProgramId)
	btcAmount := mustGetTokenAccount(chainClient, ctx, poolState.Token0Vault).Amount

	isBuy := true
	for {
		if btcAmount < 1000000 {
			// btc < 0.1, deposit more btc
			// deposit more funds for liquidity
			btcAmount := uint64(1000000)
			usdtAmount := btcAmount * InitialBtcUsdtPrice * 1000000 / uint64(100000000)
			deposit(
				chainClient,
				ctx,
				senderAddress,
				btcAmount,
				usdtAmount,
				traderBtcAta,
				traderUsdtAta,
				poolState.Token0Vault,
				poolState.Token1Vault,
				btcMint,
				usdtMint,
				senderSvmAddress,
				authorityAccount,
				poolAccount,
				poolState.LpMint,
				*poolState,
			)
		}

		// start swap
		btcDecimals := math.NewInt(100000000)
		usdtDecimals := math.NewInt(1000000)
		if isBuy {
			traderBtcBefore := mustGetTokenAccount(chainClient, ctx, traderBtcAta).Amount
			traderUsdtBefore := mustGetTokenAccount(chainClient, ctx, traderUsdtAta).Amount
			swapBaseInput(
				chainClient,
				ctx,
				senderAddress,
				raydiumProgramId,
				authorityAccount,
				2000000, // 0.02 BTC
				100_000000,
				traderBtcAta,
				traderUsdtAta,
				poolState.AmmConfig,
				poolAccount,
				poolState.Token0Vault,
				poolState.Token1Vault,
				btcMint,
				usdtMint,
				poolState.ObservationKey,
			)

			traderBtcAfter := mustGetTokenAccount(chainClient, ctx, traderBtcAta).Amount
			traderUsdtAfter := mustGetTokenAccount(chainClient, ctx, traderUsdtAta).Amount
			btcChange := math.LegacyNewDecFromInt(math.NewIntFromUint64(traderBtcAfter).Sub(math.NewIntFromUint64(traderBtcBefore)))
			usdtChange := math.LegacyNewDecFromInt(math.NewIntFromUint64(traderUsdtAfter).Sub(math.NewIntFromUint64(traderUsdtBefore)))
			btcChangeFloat, _ := btcChange.Abs().QuoInt(btcDecimals).Float64()
			usdtChangeFloat, _ := usdtChange.QuoInt(usdtDecimals).Float64()
			fmt.Println("Sold", btcChangeFloat, "BTC for", usdtChangeFloat, "USDT")
		} else {
			traderBtcBefore := mustGetTokenAccount(chainClient, ctx, traderBtcAta).Amount
			traderUsdtBefore := mustGetTokenAccount(chainClient, ctx, traderUsdtAta).Amount
			swapBaseInput(
				chainClient,
				ctx,
				senderAddress,
				raydiumProgramId,
				authorityAccount,
				400_000000,
				500000,
				traderUsdtAta,
				traderBtcAta,
				poolState.AmmConfig,
				poolAccount,
				poolState.Token1Vault,
				poolState.Token0Vault,
				usdtMint,
				btcMint,
				poolState.ObservationKey,
			)

			traderBtcAfter := mustGetTokenAccount(chainClient, ctx, traderBtcAta).Amount
			traderUsdtAfter := mustGetTokenAccount(chainClient, ctx, traderUsdtAta).Amount
			btcChange := math.LegacyNewDecFromInt(math.NewIntFromUint64(traderBtcAfter).Sub(math.NewIntFromUint64(traderBtcBefore)))
			usdtChange := math.LegacyNewDecFromInt(math.NewIntFromUint64(traderUsdtAfter).Sub(math.NewIntFromUint64(traderUsdtBefore)))
			btcChangeFloat, _ := btcChange.Abs().QuoInt(btcDecimals).Float64()
			usdtChangeFloat, _ := usdtChange.Abs().QuoInt(usdtDecimals).Float64()
			fmt.Println("Buy", btcChangeFloat, "BTC from", usdtChangeFloat, "USDT")
		}
		isBuy = !isBuy

		btcAmount := mustGetTokenAccount(chainClient, ctx, poolState.Token0Vault).Amount
		usdtAmount := mustGetTokenAccount(chainClient, ctx, poolState.Token1Vault).Amount
		fmt.Println("Vault balance:", float64(btcAmount)/100000000, "BTC /", float64(usdtAmount)/1000000, "USDT")
		time.Sleep(2 * time.Second)
	}
}
