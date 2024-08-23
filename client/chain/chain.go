package chain

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	astromeshtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"
	svmtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
	"github.com/gagliardetto/solana-go"
	"github.com/golang/protobuf/proto"

	"github.com/FluxNFTLabs/sdk-go/client/common"
	log "github.com/InjectiveLabs/suplog"
	"github.com/cosmos/cosmos-sdk/client"
	nodetypes "github.com/cosmos/cosmos-sdk/client/grpc/node"
	"github.com/cosmos/cosmos-sdk/client/tx"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cosmtypes "github.com/cosmos/cosmos-sdk/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	authztypes "github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	eth "github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	msgCommitBatchSizeLimit          = 1024
	msgCommitBatchTimeLimit          = 500 * time.Millisecond
	defaultBroadcastStatusPoll       = 100 * time.Millisecond
	defaultBroadcastTimeout          = 40 * time.Second
	defaultTimeoutHeight             = 20
	defaultTimeoutHeightSyncInterval = 10 * time.Second
	defaultSessionRenewalOffset      = 120
	defaultBlockTime                 = 3 * time.Second
	defaultChainCookieName           = ".chain_cookie"
)

var (
	ErrTimedOut       = errors.New("tx timed out")
	ErrQueueClosed    = errors.New("queue is closed")
	ErrEnqueueTimeout = errors.New("enqueue timeout")
	ErrReadOnly       = errors.New("client is in read-only mode")
)

type ChainClient interface {
	CanSignTransactions() bool
	FromAddress() sdk.AccAddress
	QueryClient() *grpc.ClientConn
	ClientContext() client.Context
	GetAccNonce() (accNum uint64, accSeq uint64)

	SimulateMsg(clientCtx client.Context, msgs ...sdk.Msg) (*txtypes.SimulateResponse, error)
	AsyncBroadcastMsg(msgs ...sdk.Msg) (*txtypes.BroadcastTxResponse, error)
	SyncBroadcastMsg(msgs ...sdk.Msg) (*txtypes.BroadcastTxResponse, error)

	BuildSignedTx(clientCtx client.Context, accNum, accSeq, initialGas uint64, msg ...sdk.Msg) (signing.Tx, error)
	SyncBroadcastSignedTx(tyBytes []byte) (*txtypes.BroadcastTxResponse, error)
	AsyncBroadcastSignedTx(txBytes []byte) (*txtypes.BroadcastTxResponse, error)
	QueueBroadcastMsg(msgs ...sdk.Msg) error

	SyncBroadcastSvmMsg(msg *svmtypes.MsgTransaction) (*txtypes.BroadcastTxResponse, error)

	GetSVMAccountLink(ctx context.Context, cosmosAddress sdk.AccAddress) (isLinked bool, pubkey solana.PublicKey, err error)
	LinkSVMAccount(svmPrivKey *ed25519.PrivKey) (*txtypes.BroadcastTxResponse, error)

	GetBankBalances(ctx context.Context, address string) (*banktypes.QueryAllBalancesResponse, error)
	GetBankBalance(ctx context.Context, address string, denom string) (*banktypes.QueryBalanceResponse, error)
	GetAuthzGrants(ctx context.Context, req authztypes.QueryGrantsRequest) (*authztypes.QueryGrantsResponse, error)
	GetAccount(ctx context.Context, address string) (*authtypes.QueryAccountResponse, error)
	GetSvmAccount(ctx context.Context, address string) (*svmtypes.AccountResponse, error)
	GetDenomLink(ctx context.Context, srcPlane astromeshtypes.Plane, denom string, dstPlane astromeshtypes.Plane) (*astromeshtypes.QueryDenomLinkResponse, error)

	BuildGenericAuthz(granter string, grantee string, msgtype string, expireIn time.Time) *authztypes.MsgGrant
	GetGasFee() (string, error)

	BroadcastDone()
	Close()
}

type chainClient struct {
	ctx       client.Context
	opts      *common.ClientOptions
	logger    log.Logger
	conn      *grpc.ClientConn
	txFactory tx.Factory

	fromAddress sdk.AccAddress
	doneC       chan bool
	msgC        chan sdk.Msg
	syncMux     *sync.Mutex

	accNum    uint64
	accSeq    uint64
	gasWanted uint64
	gasFee    string

	sessionCookie  string
	sessionEnabled bool

	nodeClient           nodetypes.ServiceClient
	txClient             txtypes.ServiceClient
	authQueryClient      authtypes.QueryClient
	bankQueryClient      banktypes.QueryClient
	authzQueryClient     authztypes.QueryClient
	svmQueryClient       svmtypes.QueryClient
	astromeshQueryClient astromeshtypes.QueryClient

	closed  int64
	canSign bool

	Broadcasted chan struct{}
}

func NewChainClient(
	ctx client.Context,
	options ...common.ClientOption,
) (ChainClient, error) {
	// process options
	opts := common.DefaultClientOptions()
	for _, opt := range options {
		if err := opt(opts); err != nil {
			err = errors.Wrap(err, "error in client option")
			return nil, err
		}
	}

	// init tx factory
	txFactory := NewTxFactory(ctx)
	if len(opts.GasPrices) > 0 {
		txFactory = txFactory.WithGasPrices(opts.GasPrices)
	}

	// build client
	cc := &chainClient{
		ctx:  ctx,
		opts: opts,

		logger: log.WithFields(log.Fields{
			"module": "sdk-go",
			"svc":    "chainClient",
		}),
		txFactory: txFactory,
		canSign:   ctx.Keyring != nil,
		syncMux:   new(sync.Mutex),
		msgC:      make(chan sdk.Msg, msgCommitBatchSizeLimit),
		doneC:     make(chan bool, 1),

		nodeClient:           nodetypes.NewServiceClient(ctx.GRPCClient),
		txClient:             txtypes.NewServiceClient(ctx.GRPCClient),
		authQueryClient:      authtypes.NewQueryClient(ctx.GRPCClient),
		bankQueryClient:      banktypes.NewQueryClient(ctx.GRPCClient),
		authzQueryClient:     authztypes.NewQueryClient(ctx.GRPCClient),
		svmQueryClient:       svmtypes.NewQueryClient(ctx.GRPCClient),
		astromeshQueryClient: astromeshtypes.NewQueryClient(ctx.GRPCClient),

		Broadcasted: make(chan struct{}, 100000000),
	}

	if cc.canSign {
		var err error
		cc.accNum, cc.accSeq, err = cc.txFactory.AccountRetriever().GetAccountNumberSequence(ctx, ctx.GetFromAddress())
		if err != nil {
			err = errors.Wrap(err, "failed to get initial account num and seq")
			return nil, err
		}

		go cc.runBatchBroadcast()
		go cc.syncTimeoutHeight()
	}

	// create file if not exist
	os.OpenFile(defaultChainCookieName, os.O_RDONLY|os.O_CREATE, 0666)

	// attempt to load from disk
	data, err := os.ReadFile(defaultChainCookieName)
	if err != nil {
		cc.logger.Errorln(err)
	} else {
		cc.sessionCookie = string(data)
		cc.logger.Infoln("chain session cookie loaded from disk")
	}

	return cc, nil
}

func (c *chainClient) syncNonce() {
	num, seq, err := c.txFactory.AccountRetriever().GetAccountNumberSequence(c.ctx, c.ctx.GetFromAddress())
	if err != nil {
		c.logger.WithError(err).Errorln("failed to get account seq")
		return
	} else if num != c.accNum {
		c.logger.WithFields(log.Fields{
			"expected": c.accNum,
			"actual":   num,
		}).Panic("account number changed during nonce sync")
	}

	c.accSeq = seq
}

func (c *chainClient) syncTimeoutHeight() {
	for {
		ctx := context.Background()
		status, err := c.nodeClient.Status(ctx, &nodetypes.StatusRequest{})
		if err != nil {
			c.logger.WithError(err).Errorln("failed to get current block")
			return
		}
		c.txFactory.WithTimeoutHeight(uint64(status.Height) + defaultTimeoutHeight)
		time.Sleep(defaultTimeoutHeightSyncInterval)
	}
}

// prepareFactory ensures the account defined by ctx.GetFromAddress() exists and
// if the account number and/or the account sequence number are zero (not set),
// they will be queried for and set on the provided Factory. A new Factory with
// the updated fields will be returned.
func (c *chainClient) prepareFactory(clientCtx client.Context, txf tx.Factory) (tx.Factory, error) {
	from := clientCtx.GetFromAddress()

	if err := txf.AccountRetriever().EnsureExists(clientCtx, from); err != nil {
		return txf, err
	}

	initNum, initSeq := txf.AccountNumber(), txf.Sequence()
	if initNum == 0 || initSeq == 0 {
		num, seq, err := txf.AccountRetriever().GetAccountNumberSequence(clientCtx, from)
		if err != nil {
			return txf, err
		}

		if initNum == 0 {
			txf = txf.WithAccountNumber(num)
		}

		if initSeq == 0 {
			txf = txf.WithSequence(seq)
		}
	}

	return txf, nil
}

func (c *chainClient) getAccSeq() uint64 {
	defer func() {
		c.accSeq += 1
	}()
	return c.accSeq
}

func (c *chainClient) setCookie(metadata metadata.MD) {
	if !c.sessionEnabled {
		return
	}
	md := metadata.Get("set-cookie")
	if len(md) > 0 {
		// write to client instance
		c.sessionCookie = md[0]
		// write to disk
		err := os.WriteFile(defaultChainCookieName, []byte(md[0]), 0644)
		if err != nil {
			c.logger.Errorln(err)
			return
		}
		c.logger.Infoln("chain session cookie saved to disk")
	}
}

func (c *chainClient) fetchCookie(ctx context.Context) context.Context {
	var header metadata.MD
	c.txClient.GetTx(context.Background(), &txtypes.GetTxRequest{}, grpc.Header(&header))
	c.setCookie(header)
	time.Sleep(defaultBlockTime)
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("cookie", c.sessionCookie))
}

func cookieByName(cookies []*http.Cookie, key string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == key {
			return c
		}
	}
	return nil
}

func (c *chainClient) getCookieExpirationTime(cookies []*http.Cookie) (time.Time, error) {
	var expiresAt string
	if cookieByName(cookies, "GCLB") != nil {
		// parse global load balance cookie timestamp
		cookie := cookieByName(cookies, "expires")
		expiresAt = strings.Replace(cookie.Value, "-", " ", -1)
	} else {
		cookie := cookieByName(cookies, "Expires")
		expiresAt = strings.Replace(cookie.Value, "-", " ", -1)
		yyyy := fmt.Sprintf("20%s", expiresAt[12:14])
		expiresAt = expiresAt[:12] + yyyy + expiresAt[14:]
	}

	return time.Parse(time.RFC1123, expiresAt)
}

func (c *chainClient) getCookie(ctx context.Context) context.Context {
	md := metadata.Pairs("cookie", c.sessionCookie)
	if !c.sessionEnabled {
		return metadata.NewOutgoingContext(ctx, md)
	}

	// borrow http request to parse cookie
	header := http.Header{}
	header.Add("Cookie", c.sessionCookie)
	request := http.Request{Header: header}
	cookies := request.Cookies()

	if len(cookies) > 0 {
		// parse expire field into unix timestamp
		expiresTimestamp, err := c.getCookieExpirationTime(cookies)
		if err != nil {
			panic(err)
		}

		// renew session if timestamp diff < offset
		timestampDiff := expiresTimestamp.Unix() - time.Now().Unix()
		if timestampDiff < defaultSessionRenewalOffset {
			return c.fetchCookie(ctx)
		}
	} else {
		return c.fetchCookie(ctx)
	}

	return metadata.NewOutgoingContext(ctx, md)
}

func (c *chainClient) GetAccNonce() (accNum uint64, accSeq uint64) {
	return c.accNum, c.accSeq
}

func (c *chainClient) QueryClient() *grpc.ClientConn {
	return c.conn
}

func (c *chainClient) ClientContext() client.Context {
	return c.ctx
}

func (c *chainClient) CanSignTransactions() bool {
	return c.canSign
}

func (c *chainClient) FromAddress() sdk.AccAddress {
	if !c.canSign {
		return sdk.AccAddress{}
	}

	return c.ctx.FromAddress
}

func (c *chainClient) Close() {
	if !c.canSign {
		return
	}
	if atomic.CompareAndSwapInt64(&c.closed, 0, 1) {
		close(c.msgC)
	}
	<-c.doneC
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *chainClient) GetBankBalances(ctx context.Context, address string) (*banktypes.QueryAllBalancesResponse, error) {
	req := &banktypes.QueryAllBalancesRequest{
		Address: address,
	}
	return c.bankQueryClient.AllBalances(ctx, req)
}

func (c *chainClient) GetAccount(ctx context.Context, address string) (*authtypes.QueryAccountResponse, error) {
	req := &authtypes.QueryAccountRequest{
		Address: address,
	}
	return c.authQueryClient.Account(ctx, req)
}

func (c *chainClient) GetSvmAccount(ctx context.Context, address string) (*svmtypes.AccountResponse, error) {
	req := &svmtypes.AccountRequest{
		Address: address,
	}
	return c.svmQueryClient.Account(ctx, req)
}

func (c *chainClient) GetDenomLink(ctx context.Context, srcPlane astromeshtypes.Plane, denom string, dstPlane astromeshtypes.Plane) (*astromeshtypes.QueryDenomLinkResponse, error) {
	req := &astromeshtypes.QueryDenomLinkRequest{
		SrcPlane: srcPlane,
		DstPlane: dstPlane,
		SrcAddr:  denom,
	}
	return c.astromeshQueryClient.DenomLink(ctx, req)
}

func (c *chainClient) GetBankBalance(ctx context.Context, address string, denom string) (*banktypes.QueryBalanceResponse, error) {
	req := &banktypes.QueryBalanceRequest{
		Address: address,
		Denom:   denom,
	}
	return c.bankQueryClient.Balance(ctx, req)
}

// SyncBroadcastMsg sends Tx to chain and waits until Tx is included in block.
func (c *chainClient) SyncBroadcastMsg(msgs ...sdk.Msg) (*txtypes.BroadcastTxResponse, error) {
	c.syncMux.Lock()
	defer c.syncMux.Unlock()

	c.txFactory = c.txFactory.WithSequence(c.accSeq)
	c.txFactory = c.txFactory.WithAccountNumber(c.accNum)
	res, err := c.broadcastTx(c.ctx, c.txFactory, true, msgs...)

	if err != nil {
		if strings.Contains(err.Error(), "account sequence mismatch") {
			c.syncNonce()
			c.txFactory = c.txFactory.WithSequence(c.accSeq)
			c.txFactory = c.txFactory.WithAccountNumber(c.accNum)
			log.Debugln("retrying broadcastTx with nonce", c.accSeq)
			res, err = c.broadcastTx(c.ctx, c.txFactory, true, msgs...)
		}
		if err != nil {
			resJSON, _ := json.MarshalIndent(res, "", "\t")
			c.logger.WithField("size", len(msgs)).WithError(err).Errorln("failed to commit msg batch:", string(resJSON))
			return nil, err
		}
	}

	c.accSeq++

	return res, nil
}

func (c *chainClient) SimulateMsg(clientCtx client.Context, msgs ...sdk.Msg) (*txtypes.SimulateResponse, error) {
	c.txFactory = c.txFactory.WithSequence(c.accSeq)
	c.txFactory = c.txFactory.WithAccountNumber(c.accNum)
	txf, err := c.prepareFactory(clientCtx, c.txFactory)
	if err != nil {
		err = errors.Wrap(err, "failed to prepareFactory")
		return nil, err
	}

	simTxBytes, err := txf.BuildSimTx(msgs...)
	if err != nil {
		err = errors.Wrap(err, "failed to build sim tx bytes")
		return nil, err
	}

	ctx := context.Background()
	ctx = c.getCookie(ctx)
	var header metadata.MD
	simRes, err := c.txClient.Simulate(ctx, &txtypes.SimulateRequest{TxBytes: simTxBytes}, grpc.Header(&header))
	if err != nil {
		err = errors.Wrap(err, "failed to CalculateGas")
		return nil, err
	}

	return simRes, nil
}

// AsyncBroadcastMsg sends Tx to chain and doesn't wait until Tx is included in block. This method
// cannot be used for rapid Tx sending, it is expected that you wait for transaction status with
// external tools. If you want sdk to wait for it, use SyncBroadcastMsg.
func (c *chainClient) AsyncBroadcastMsg(msgs ...sdk.Msg) (*txtypes.BroadcastTxResponse, error) {
	c.syncMux.Lock()
	defer c.syncMux.Unlock()

	c.txFactory = c.txFactory.WithSequence(c.accSeq)
	c.txFactory = c.txFactory.WithAccountNumber(c.accNum)
	res, err := c.broadcastTx(c.ctx, c.txFactory, false, msgs...)
	if err != nil {
		if strings.Contains(err.Error(), "account sequence mismatch") {
			c.syncNonce()
			c.txFactory = c.txFactory.WithSequence(c.accSeq)
			c.txFactory = c.txFactory.WithAccountNumber(c.accNum)
			log.Debugln("retrying broadcastTx with nonce", c.accSeq)
			res, err = c.broadcastTx(c.ctx, c.txFactory, false, msgs...)
		}
		if err != nil {
			resJSON, _ := json.MarshalIndent(res, "", "\t")
			c.logger.WithField("size", len(msgs)).WithError(err).Errorln("failed to commit msg batch:", string(resJSON))
			return nil, err
		}
	}

	c.accSeq++

	return res, nil
}

func (c *chainClient) BuildSignedTx(clientCtx client.Context, accNum, accSeq, initialGas uint64, msgs ...sdk.Msg) (signing.Tx, error) {
	ctx := context.Background()
	txf := NewTxFactory(clientCtx).WithSequence(accSeq).WithAccountNumber(accNum).WithGas(initialGas)

	if clientCtx.Simulate {
		simTxBytes, err := txf.BuildSimTx(msgs...)
		if err != nil {
			err = errors.Wrap(err, "failed to build sim tx bytes")
			return nil, err
		}
		ctx := c.getCookie(context.Background())
		var header metadata.MD
		simRes, err := c.txClient.Simulate(ctx, &txtypes.SimulateRequest{TxBytes: simTxBytes}, grpc.Header(&header))
		if err != nil {
			err = errors.Wrap(err, "failed to CalculateGas")
			return nil, err
		}

		adjustedGas := uint64(txf.GasAdjustment() * float64(simRes.GasInfo.GasUsed))
		txf = txf.WithGas(adjustedGas)

		c.gasWanted = adjustedGas
	}

	txf, err := c.prepareFactory(clientCtx, txf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepareFactory")
	}

	txn, err := txf.BuildUnsignedTx(msgs...)
	if err != nil {
		err = errors.Wrap(err, "failed to BuildUnsignedTx")
		return nil, err
	}

	txn.SetFeeGranter(clientCtx.GetFeeGranterAddress())
	err = tx.Sign(ctx, txf, clientCtx.GetFromName(), txn, true)
	if err != nil {
		err = errors.Wrap(err, "failed to Sign Tx")
		return nil, err
	}

	return txn.GetTx(), nil
}

func (c *chainClient) SyncBroadcastSignedTx(txBytes []byte) (*txtypes.BroadcastTxResponse, error) {
	req := txtypes.BroadcastTxRequest{
		TxBytes: txBytes,
		Mode:    txtypes.BroadcastMode_BROADCAST_MODE_SYNC,
	}

	ctx := context.Background()
	var header metadata.MD
	ctx = c.getCookie(ctx)

	res, err := c.txClient.BroadcastTx(ctx, &req, grpc.Header(&header))
	if err != nil || res.TxResponse.Code != 0 {
		return res, err
	}

	awaitCtx, cancelFn := context.WithTimeout(context.Background(), defaultBroadcastTimeout)
	defer cancelFn()

	t := time.NewTimer(defaultBroadcastStatusPoll)

	for {
		select {
		case <-awaitCtx.Done():
			err := errors.Wrapf(ErrTimedOut, "%s", res.TxResponse.TxHash)
			t.Stop()
			return nil, err
		case <-t.C:
			resultTx, err := c.txClient.GetTx(awaitCtx, &txtypes.GetTxRequest{Hash: res.TxResponse.TxHash})
			if err != nil {
				t.Reset(defaultBroadcastStatusPoll)
				continue
			} else if resultTx.TxResponse.Height > 0 {
				res = &txtypes.BroadcastTxResponse{TxResponse: resultTx.TxResponse}
				t.Stop()
				return res, err
			}

			t.Reset(defaultBroadcastStatusPoll)
		}
	}
}

func (c *chainClient) AsyncBroadcastSignedTx(txBytes []byte) (*txtypes.BroadcastTxResponse, error) {
	req := txtypes.BroadcastTxRequest{
		TxBytes: txBytes,
		Mode:    txtypes.BroadcastMode_BROADCAST_MODE_SYNC,
	}

	ctx := context.Background()
	// use our own client to broadcast tx
	var header metadata.MD
	ctx = c.getCookie(ctx)
	res, err := c.txClient.BroadcastTx(ctx, &req, grpc.Header(&header))
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *chainClient) broadcastTx(
	clientCtx client.Context,
	txf tx.Factory,
	await bool,
	msgs ...sdk.Msg,
) (*txtypes.BroadcastTxResponse, error) {
	ctx := context.Background()
	txf, err := c.prepareFactory(clientCtx, txf)

	if err != nil {
		err = errors.Wrap(err, "failed to prepareFactory")
		return nil, err
	}
	if clientCtx.Simulate {
		simTxBytes, err := txf.BuildSimTx(msgs...)
		if err != nil {
			err = errors.Wrap(err, "failed to build sim tx bytes")
			return nil, err
		}
		ctx := c.getCookie(ctx)
		var header metadata.MD
		simRes, err := c.txClient.Simulate(ctx, &txtypes.SimulateRequest{TxBytes: simTxBytes}, grpc.Header(&header))
		if err != nil {
			err = errors.Wrap(err, "failed to CalculateGas")
			return nil, err
		}

		adjustedGas := uint64(txf.GasAdjustment() * float64(simRes.GasInfo.GasUsed))
		txf = txf.WithGas(adjustedGas)

		c.gasWanted = adjustedGas
	}

	txn, err := txf.BuildUnsignedTx(msgs...)

	if err != nil {
		err = errors.Wrap(err, "failed to BuildUnsignedTx")
		return nil, err
	}

	txn.SetFeeGranter(clientCtx.GetFeeGranterAddress())
	err = tx.Sign(ctx, txf, clientCtx.GetFromName(), txn, true)
	if err != nil {
		err = errors.Wrap(err, "failed to Sign Tx")
		return nil, err
	}

	txBytes, err := clientCtx.TxConfig.TxEncoder()(txn.GetTx())
	if err != nil {
		err = errors.Wrap(err, "failed TxEncoder to encode Tx")
		return nil, err
	}

	req := txtypes.BroadcastTxRequest{
		txBytes,
		txtypes.BroadcastMode_BROADCAST_MODE_SYNC,
	}
	// use our own client to broadcast tx
	var header metadata.MD
	ctx = c.getCookie(ctx)
	res, err := c.txClient.BroadcastTx(ctx, &req, grpc.Header(&header))
	if err != nil || res.TxResponse.Code != 0 || !await {
		return res, err
	}

	awaitCtx, cancelFn := context.WithTimeout(context.Background(), defaultBroadcastTimeout)
	defer cancelFn()

	t := time.NewTimer(defaultBroadcastStatusPoll)

	for {
		select {
		case <-awaitCtx.Done():
			err := errors.Wrapf(ErrTimedOut, "%s", res.TxResponse.TxHash)
			t.Stop()
			return nil, err
		case <-t.C:
			resultTx, err := c.txClient.GetTx(awaitCtx, &txtypes.GetTxRequest{Hash: res.TxResponse.TxHash})
			if err != nil {
				t.Reset(defaultBroadcastStatusPoll)
				continue
			} else if resultTx.TxResponse.Height > 0 {
				res = &txtypes.BroadcastTxResponse{TxResponse: resultTx.TxResponse}
				t.Stop()
				return res, err
			}

			t.Reset(defaultBroadcastStatusPoll)
		}
	}
}

// QueueBroadcastMsg enqueues a list of messages. Messages will added to the queue
// and grouped into Txns in chunks. Use this method to mass broadcast Txns with efficiency.
func (c *chainClient) QueueBroadcastMsg(msgs ...sdk.Msg) error {
	if !c.canSign {
		return ErrReadOnly
	} else if atomic.LoadInt64(&c.closed) == 1 {
		return ErrQueueClosed
	}

	t := time.NewTimer(10 * time.Second)
	for _, msg := range msgs {
		select {
		case <-t.C:
			return ErrEnqueueTimeout
		case c.msgC <- msg:
		}
	}
	t.Stop()

	return nil
}

func (c *chainClient) runBatchBroadcast() {
	expirationTimer := time.NewTimer(msgCommitBatchTimeLimit)
	msgBatch := make([]sdk.Msg, 0, msgCommitBatchSizeLimit)

	submitBatch := func(toSubmit []sdk.Msg) {
		c.syncMux.Lock()
		defer c.syncMux.Unlock()
		c.txFactory = c.txFactory.WithSequence(c.accSeq)
		c.txFactory = c.txFactory.WithAccountNumber(c.accNum)
		log.Debugln("broadcastTx with nonce", c.accSeq)
		res, err := c.broadcastTx(c.ctx, c.txFactory, true, toSubmit...)
		if err != nil {
			if strings.Contains(err.Error(), "account sequence mismatch") {
				c.syncNonce()
				c.txFactory = c.txFactory.WithSequence(c.accSeq)
				c.txFactory = c.txFactory.WithAccountNumber(c.accNum)
				log.Debugln("retrying broadcastTx with nonce", c.accSeq)
				res, err = c.broadcastTx(c.ctx, c.txFactory, true, toSubmit...)
			}
			if err != nil {
				resJSON, _ := json.MarshalIndent(res, "", "\t")
				c.logger.WithField("size", len(toSubmit)).WithError(err).Errorln("failed to commit msg batch:", string(resJSON))
				return
			}
		}

		if res.TxResponse.Code != 0 {
			err = errors.Errorf("error %d (%s): %s", res.TxResponse.Code, res.TxResponse.Codespace, res.TxResponse.RawLog)
			log.WithField("txHash", res.TxResponse.TxHash).WithError(err).Errorln("failed to commit msg batch")
		} else {
			log.WithField("txHash", res.TxResponse.TxHash).Debugln("msg batch committed successfully at height", res.TxResponse.Height)
		}

		c.accSeq++
		log.Debugln("nonce incremented to", c.accSeq)
		log.Debugln("gas wanted: ", c.gasWanted)
		log.Debugln("gas used: ", res.TxResponse.GasUsed)

		c.Broadcasted <- struct{}{}
	}

	for {
		select {
		case msg, ok := <-c.msgC:
			if !ok {
				// exit required
				if len(msgBatch) > 0 {
					submitBatch(msgBatch)
				}

				close(c.doneC)
				return
			}

			msgBatch = append(msgBatch, msg)

			if len(msgBatch) >= msgCommitBatchSizeLimit {
				toSubmit := msgBatch
				msgBatch = msgBatch[:0]
				expirationTimer.Reset(msgCommitBatchTimeLimit)

				submitBatch(toSubmit)
			}
		case <-expirationTimer.C:
			if len(msgBatch) > 0 {
				toSubmit := msgBatch
				msgBatch = msgBatch[:0]
				expirationTimer.Reset(msgCommitBatchTimeLimit)
				submitBatch(toSubmit)
			} else {
				expirationTimer.Reset(msgCommitBatchTimeLimit)
			}
		}
	}
}

func (c *chainClient) GetGasFee() (string, error) {
	gasPrices := strings.Trim(c.opts.GasPrices, "lux")

	gas, err := strconv.ParseFloat(gasPrices, 64)

	if err != nil {
		return "", err
	}

	gasFeeAdjusted := gas * float64(c.gasWanted) / math.Pow(10, 18)
	gasFeeFormatted := strconv.FormatFloat(gasFeeAdjusted, 'f', -1, 64)
	c.gasFee = gasFeeFormatted

	return c.gasFee, err
}

func (c *chainClient) DefaultSubaccount(acc cosmtypes.AccAddress) eth.Hash {
	return eth.BytesToHash(eth.RightPadBytes(acc.Bytes(), 32))
}

func (c *chainClient) GetAuthzGrants(ctx context.Context, req authztypes.QueryGrantsRequest) (*authztypes.QueryGrantsResponse, error) {
	return c.authzQueryClient.Grants(ctx, &req)
}

func (c *chainClient) BuildGenericAuthz(granter string, grantee string, msgtype string, expireIn time.Time) *authztypes.MsgGrant {
	authz := authztypes.NewGenericAuthorization(msgtype)
	authzAny := codectypes.UnsafePackAny(authz)
	return &authztypes.MsgGrant{
		Granter: granter,
		Grantee: grantee,
		Grant: authztypes.Grant{
			Authorization: authzAny,
			Expiration:    &expireIn,
		},
	}
}

func (c *chainClient) BroadcastDone() {
	<-c.Broadcasted
}

func (c *chainClient) SyncBroadcastSvmMsg(msg *svmtypes.MsgTransaction) (*txtypes.BroadcastTxResponse, error) {
	c.txFactory = c.txFactory.WithSequence(c.accSeq)
	c.txFactory = c.txFactory.WithAccountNumber(c.accNum)
	txf, err := c.prepareFactory(c.ClientContext(), c.txFactory)
	if err != nil {
		err = errors.Wrap(err, "failed to prepareFactory")
		return nil, err
	}

	simTxBytes, err := txf.BuildSimTx(msg)
	if err != nil {
		err = errors.Wrap(err, "failed to build sim tx bytes")
		return nil, err
	}

	// simulate
	ctx := context.Background()
	ctx = c.getCookie(ctx)
	var header metadata.MD
	simRes, err := c.txClient.Simulate(ctx, &txtypes.SimulateRequest{TxBytes: simTxBytes}, grpc.Header(&header))
	if err != nil {
		err = errors.Wrap(err, "failed to CalculateGas")
		return nil, err
	}

	// adjust msg compute budget
	var msgRes svmtypes.MsgTransactionResponse
	err = proto.Unmarshal(simRes.Result.MsgResponses[0].Value, &msgRes)
	if err != nil {
		panic(err)
	}
	msg.ComputeBudget = msgRes.UnitConsumed * 2

	// adjust gas
	adjustedGas := uint64(txf.GasAdjustment() * float64(simRes.GasInfo.GasUsed))
	txf = txf.WithGas(adjustedGas)
	c.gasWanted = adjustedGas

	// broadcast
	txn, err := txf.BuildUnsignedTx(msg)
	if err != nil {
		err = errors.Wrap(err, "failed to BuildUnsignedTx")
		return nil, err
	}
	txn.SetFeeGranter(c.ClientContext().GetFeeGranterAddress())
	err = tx.Sign(ctx, txf, c.ClientContext().GetFromName(), txn, true)
	if err != nil {
		err = errors.Wrap(err, "failed to Sign Tx")
		return nil, err
	}
	txBytes, err := c.ClientContext().TxConfig.TxEncoder()(txn.GetTx())
	if err != nil {
		err = errors.Wrap(err, "failed TxEncoder to encode Tx")
		return nil, err
	}
	req := txtypes.BroadcastTxRequest{
		TxBytes: txBytes,
		Mode:    txtypes.BroadcastMode_BROADCAST_MODE_SYNC,
	}
	res, err := c.txClient.BroadcastTx(ctx, &req, grpc.Header(&header))
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *chainClient) GetSVMAccountLink(ctx context.Context, cosmosAddress sdk.AccAddress) (isLinked bool, pubkey solana.PublicKey, err error) {
	resp, err := c.svmQueryClient.AccountLink(context.Background(), &svmtypes.AccountLinkRequest{
		Address: cosmosAddress.String(),
	})

	if err != nil {
		if !strings.Contains(err.Error(), "account link not found") {
			return false, solana.PublicKey{}, err
		}
		return false, solana.PublicKey{}, nil
	}

	pubkey = solana.PublicKeyFromBytes(resp.Link.SvmAddr)
	if err != nil {
		return true, pubkey, fmt.Errorf("parse base58 pubkey err: %w", err)
	}

	return true, pubkey, nil
}

func (c *chainClient) LinkSVMAccount(svmPrivKey *ed25519.PrivKey) (*txtypes.BroadcastTxResponse, error) {
	svmAccountLinkSig, err := svmPrivKey.Sign(c.FromAddress().Bytes())
	if err != nil {
		return nil, fmt.Errorf("svm privkey sign err: %w", err)
	}

	return c.SyncBroadcastMsg(&svmtypes.MsgLinkSVMAccount{
		Sender:       c.FromAddress().String(),
		SvmPubkey:    svmPrivKey.PubKey().Bytes(),
		SvmSignature: svmAccountLinkSig,
		Amount:       sdk.NewInt64Coin("lux", 1_000_000_000_000_000),
	})
}
