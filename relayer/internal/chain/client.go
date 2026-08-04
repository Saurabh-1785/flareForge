// Package chain provides Ethereum/Flare chain interaction for the relayer.
// It watches VaultRegistry events on Coston2 and sends transactions
// (markWarning, requestAttestation, submitQuorumResult, finalizeDisputeWindow,
// finalizeFinalWindow) when deadlines expire.
//
// Build prompt: "Goroutine-per-vault deadline watcher against your Coston2
// contract's events" using "go-ethereum's ethclient for event watching".
package chain

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.uber.org/zap"
)

// ──────────────────────────────────────────────────────────────────────
// ABI definitions — matching VaultRegistry.sol events and functions
// ──────────────────────────────────────────────────────────────────────

// VaultRegistryABI is the minimal ABI for the events and functions
// the relayer needs to interact with.
const VaultRegistryABI = `[
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "vaultId", "type": "uint256"},
			{"indexed": true, "name": "owner", "type": "address"},
			{"indexed": false, "name": "planCommitmentHash", "type": "bytes32"}
		],
		"name": "VaultCreated",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "vaultId", "type": "uint256"},
			{"indexed": false, "name": "nextDeadline", "type": "uint256"}
		],
		"name": "CheckIn",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "vaultId", "type": "uint256"},
			{"indexed": false, "name": "from", "type": "uint8"},
			{"indexed": false, "name": "to", "type": "uint8"}
		],
		"name": "StateTransition",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "vaultId", "type": "uint256"}
		],
		"name": "AttestationRequested",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "vaultId", "type": "uint256"},
			{"indexed": false, "name": "quorumMet", "type": "bool"}
		],
		"name": "QuorumResultSubmitted",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "vaultId", "type": "uint256"},
			{"indexed": false, "name": "guardian", "type": "address"}
		],
		"name": "GuardianHalt",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "vaultId", "type": "uint256"}
		],
		"name": "VaultFullyReleased",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "vaultId", "type": "uint256"}
		],
		"name": "VaultCancelled",
		"type": "event"
	},
	{
		"inputs": [{"name": "vaultId", "type": "uint256"}],
		"name": "markWarning",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [{"name": "vaultId", "type": "uint256"}],
		"name": "requestAttestation",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{"name": "vaultId", "type": "uint256"},
			{"name": "quorumMet", "type": "bool"},
			{"name": "fceSignature", "type": "bytes"}
		],
		"name": "submitQuorumResult",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [{"name": "vaultId", "type": "uint256"}],
		"name": "finalizeDisputeWindow",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [{"name": "vaultId", "type": "uint256"}],
		"name": "finalizeFinalWindow",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [{"name": "vaultId", "type": "uint256"}],
		"name": "isCheckInMissed",
		"outputs": [{"name": "", "type": "bool"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [{"name": "vaultId", "type": "uint256"}],
		"name": "getVaultState",
		"outputs": [{"name": "", "type": "uint8"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [{"name": "vaultId", "type": "uint256"}],
		"name": "getVaultOwner",
		"outputs": [{"name": "", "type": "address"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [{"name": "vaultId", "type": "uint256"}],
		"name": "getVaultTiming",
		"outputs": [
			{"name": "lastCheckIn", "type": "uint256"},
			{"name": "windowDeadline", "type": "uint256"},
			{"name": "checkInInterval", "type": "uint256"},
			{"name": "graceWindow", "type": "uint256"},
			{"name": "disputeWindow", "type": "uint256"},
			{"name": "finalWindow", "type": "uint256"}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "nextVaultId",
		"outputs": [{"name": "", "type": "uint256"}],
		"stateMutability": "view",
		"type": "function"
	}
]`

// Client wraps an Ethereum client for VaultRegistry interaction.
type Client struct {
	eth           *ethclient.Client
	abi           abi.ABI
	registryAddr  common.Address
	relayerKey    *ecdsa.PrivateKey
	relayerAddr   common.Address
	chainID       *big.Int
	logger        *zap.Logger
}

// ClientConfig holds configuration for the chain client.
type ClientConfig struct {
	RPCURL          string
	RegistryAddress string
	RelayerKeyHex   string // hex-encoded private key (without 0x prefix)
	ChainID         int64
	Logger          *zap.Logger
}

// NewClient creates a new chain client connected to Coston2.
func NewClient(cfg ClientConfig) (*Client, error) {
	logger := cfg.Logger
	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	// Connect to RPC
	eth, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RPC %s: %w", cfg.RPCURL, err)
	}

	// Parse ABI
	parsedABI, err := abi.JSON(strings.NewReader(VaultRegistryABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	// Parse relayer private key
	key, err := crypto.HexToECDSA(cfg.RelayerKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse relayer key: %w", err)
	}
	relayerAddr := crypto.PubkeyToAddress(key.PublicKey)

	return &Client{
		eth:          eth,
		abi:          parsedABI,
		registryAddr: common.HexToAddress(cfg.RegistryAddress),
		relayerKey:   key,
		relayerAddr:  relayerAddr,
		chainID:      big.NewInt(cfg.ChainID),
		logger:       logger,
	}, nil
}

// RelayerAddress returns the relayer's Ethereum address.
func (c *Client) RelayerAddress() string {
	return c.relayerAddr.Hex()
}

// ──────────────────────────────────────────────────────────────────────
// Read operations — view functions
// ──────────────────────────────────────────────────────────────────────

// GetVaultState reads the current on-chain state for a vault.
func (c *Client) GetVaultState(ctx context.Context, vaultID uint64) (uint8, error) {
	data, err := c.abi.Pack("getVaultState", new(big.Int).SetUint64(vaultID))
	if err != nil {
		return 0, fmt.Errorf("pack getVaultState: %w", err)
	}

	result, err := c.eth.CallContract(ctx, ethereum.CallMsg{
		To:   &c.registryAddr,
		Data: data,
	}, nil)
	if err != nil {
		return 0, fmt.Errorf("call getVaultState: %w", err)
	}

	outputs, err := c.abi.Unpack("getVaultState", result)
	if err != nil {
		return 0, fmt.Errorf("unpack getVaultState: %w", err)
	}
	return outputs[0].(uint8), nil
}

// GetVaultOwner reads the vault owner address.
func (c *Client) GetVaultOwner(ctx context.Context, vaultID uint64) (string, error) {
	data, err := c.abi.Pack("getVaultOwner", new(big.Int).SetUint64(vaultID))
	if err != nil {
		return "", fmt.Errorf("pack getVaultOwner: %w", err)
	}

	result, err := c.eth.CallContract(ctx, ethereum.CallMsg{
		To:   &c.registryAddr,
		Data: data,
	}, nil)
	if err != nil {
		return "", fmt.Errorf("call getVaultOwner: %w", err)
	}

	outputs, err := c.abi.Unpack("getVaultOwner", result)
	if err != nil {
		return "", fmt.Errorf("unpack getVaultOwner: %w", err)
	}
	return outputs[0].(common.Address).Hex(), nil
}

// VaultTiming holds the timing fields from the on-chain vault.
type VaultTiming struct {
	LastCheckIn     uint64
	WindowDeadline  uint64
	CheckInInterval uint64
	GraceWindow     uint64
	DisputeWindow   uint64
	FinalWindow     uint64
}

// GetVaultTiming reads the timing fields for a vault.
func (c *Client) GetVaultTiming(ctx context.Context, vaultID uint64) (*VaultTiming, error) {
	data, err := c.abi.Pack("getVaultTiming", new(big.Int).SetUint64(vaultID))
	if err != nil {
		return nil, fmt.Errorf("pack getVaultTiming: %w", err)
	}

	result, err := c.eth.CallContract(ctx, ethereum.CallMsg{
		To:   &c.registryAddr,
		Data: data,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("call getVaultTiming: %w", err)
	}

	outputs, err := c.abi.Unpack("getVaultTiming", result)
	if err != nil {
		return nil, fmt.Errorf("unpack getVaultTiming: %w", err)
	}

	return &VaultTiming{
		LastCheckIn:     outputs[0].(*big.Int).Uint64(),
		WindowDeadline:  outputs[1].(*big.Int).Uint64(),
		CheckInInterval: outputs[2].(*big.Int).Uint64(),
		GraceWindow:     outputs[3].(*big.Int).Uint64(),
		DisputeWindow:   outputs[4].(*big.Int).Uint64(),
		FinalWindow:     outputs[5].(*big.Int).Uint64(),
	}, nil
}

// IsCheckInMissed checks whether the vault's check-in is overdue.
func (c *Client) IsCheckInMissed(ctx context.Context, vaultID uint64) (bool, error) {
	data, err := c.abi.Pack("isCheckInMissed", new(big.Int).SetUint64(vaultID))
	if err != nil {
		return false, fmt.Errorf("pack isCheckInMissed: %w", err)
	}

	result, err := c.eth.CallContract(ctx, ethereum.CallMsg{
		To:   &c.registryAddr,
		Data: data,
	}, nil)
	if err != nil {
		return false, fmt.Errorf("call isCheckInMissed: %w", err)
	}

	outputs, err := c.abi.Unpack("isCheckInMissed", result)
	if err != nil {
		return false, fmt.Errorf("unpack isCheckInMissed: %w", err)
	}
	return outputs[0].(bool), nil
}

// GetNextVaultID reads the next vault ID counter (to know how many vaults exist).
func (c *Client) GetNextVaultID(ctx context.Context) (uint64, error) {
	data, err := c.abi.Pack("nextVaultId")
	if err != nil {
		return 0, fmt.Errorf("pack nextVaultId: %w", err)
	}

	result, err := c.eth.CallContract(ctx, ethereum.CallMsg{
		To:   &c.registryAddr,
		Data: data,
	}, nil)
	if err != nil {
		return 0, fmt.Errorf("call nextVaultId: %w", err)
	}

	outputs, err := c.abi.Unpack("nextVaultId", result)
	if err != nil {
		return 0, fmt.Errorf("unpack nextVaultId: %w", err)
	}
	return outputs[0].(*big.Int).Uint64(), nil
}

// ──────────────────────────────────────────────────────────────────────
// Write operations — state-changing transactions
// ──────────────────────────────────────────────────────────────────────

// sendTx is a helper that packs calldata, builds a tx, signs it, and sends it.
func (c *Client) sendTx(ctx context.Context, method string, args ...interface{}) (*types.Transaction, error) {
	data, err := c.abi.Pack(method, args...)
	if err != nil {
		return nil, fmt.Errorf("pack %s: %w", method, err)
	}

	nonce, err := c.eth.PendingNonceAt(ctx, c.relayerAddr)
	if err != nil {
		return nil, fmt.Errorf("get nonce: %w", err)
	}

	gasPrice, err := c.eth.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("get gas price: %w", err)
	}

	gasLimit, err := c.eth.EstimateGas(ctx, ethereum.CallMsg{
		From: c.relayerAddr,
		To:   &c.registryAddr,
		Data: data,
	})
	if err != nil {
		// Fallback to a generous limit
		gasLimit = 300000
		c.logger.Warn("gas estimation failed, using fallback",
			zap.String("method", method),
			zap.Error(err),
		)
	}

	tx := types.NewTransaction(nonce, c.registryAddr, big.NewInt(0), gasLimit, gasPrice, data)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(c.chainID), c.relayerKey)
	if err != nil {
		return nil, fmt.Errorf("sign tx: %w", err)
	}

	if err := c.eth.SendTransaction(ctx, signedTx); err != nil {
		return nil, fmt.Errorf("send tx %s: %w", method, err)
	}

	c.logger.Info("transaction sent",
		zap.String("method", method),
		zap.String("txHash", signedTx.Hash().Hex()),
	)
	return signedTx, nil
}

// MarkWarning calls markWarning(vaultId) — ACTIVE → WARNING when check-in missed.
func (c *Client) MarkWarning(ctx context.Context, vaultID uint64) (*types.Transaction, error) {
	return c.sendTx(ctx, "markWarning", new(big.Int).SetUint64(vaultID))
}

// RequestAttestation calls requestAttestation(vaultId) — WARNING → QUORUM_PENDING.
func (c *Client) RequestAttestation(ctx context.Context, vaultID uint64) (*types.Transaction, error) {
	return c.sendTx(ctx, "requestAttestation", new(big.Int).SetUint64(vaultID))
}

// SubmitQuorumResult calls submitQuorumResult(vaultId, quorumMet, signature).
func (c *Client) SubmitQuorumResult(ctx context.Context, vaultID uint64, quorumMet bool, signature []byte) (*types.Transaction, error) {
	return c.sendTx(ctx, "submitQuorumResult",
		new(big.Int).SetUint64(vaultID),
		quorumMet,
		signature,
	)
}

// FinalizeDisputeWindow calls finalizeDisputeWindow(vaultId).
func (c *Client) FinalizeDisputeWindow(ctx context.Context, vaultID uint64) (*types.Transaction, error) {
	return c.sendTx(ctx, "finalizeDisputeWindow", new(big.Int).SetUint64(vaultID))
}

// FinalizeFinalWindow calls finalizeFinalWindow(vaultId).
func (c *Client) FinalizeFinalWindow(ctx context.Context, vaultID uint64) (*types.Transaction, error) {
	return c.sendTx(ctx, "finalizeFinalWindow", new(big.Int).SetUint64(vaultID))
}

// WaitForReceipt waits for a transaction to be mined and returns the receipt.
func (c *Client) WaitForReceipt(ctx context.Context, tx *types.Transaction, timeout time.Duration) (*types.Receipt, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	receipt, err := bind.WaitMined(ctx, c.eth, tx)
	if err != nil {
		return nil, fmt.Errorf("wait for receipt: %w", err)
	}
	return receipt, nil
}

// ──────────────────────────────────────────────────────────────────────
// Event subscription
// ──────────────────────────────────────────────────────────────────────

// SubscribeEvents creates a log subscription for VaultRegistry events.
func (c *Client) SubscribeEvents(ctx context.Context) (ethereum.Subscription, chan types.Log, error) {
	query := ethereum.FilterQuery{
		Addresses: []common.Address{c.registryAddr},
	}

	logCh := make(chan types.Log, 100)
	sub, err := c.eth.SubscribeFilterLogs(ctx, query, logCh)
	if err != nil {
		return nil, nil, fmt.Errorf("subscribe to events: %w", err)
	}
	return sub, logCh, nil
}

// ParsedEvent represents a decoded VaultRegistry event.
type ParsedEvent struct {
	Name    string
	VaultID uint64
	Raw     types.Log
	// Event-specific fields
	FromState uint8  // StateTransition
	ToState   uint8  // StateTransition
	Owner     string // VaultCreated
	QuorumMet bool   // QuorumResultSubmitted
}

// ParseEvent decodes a raw log into a ParsedEvent.
func (c *Client) ParseEvent(log types.Log) (*ParsedEvent, error) {
	if len(log.Topics) == 0 {
		return nil, fmt.Errorf("log has no topics")
	}

	event, err := c.abi.EventByID(log.Topics[0])
	if err != nil {
		return nil, fmt.Errorf("unknown event: %w", err)
	}

	parsed := &ParsedEvent{
		Name: event.Name,
		Raw:  log,
	}

	// Extract indexed vaultId from Topics[1]
	if len(log.Topics) > 1 {
		parsed.VaultID = new(big.Int).SetBytes(log.Topics[1].Bytes()).Uint64()
	}

	// Decode non-indexed fields
	switch event.Name {
	case "StateTransition":
		if len(log.Data) >= 64 {
			parsed.FromState = uint8(new(big.Int).SetBytes(log.Data[:32]).Uint64())
			parsed.ToState = uint8(new(big.Int).SetBytes(log.Data[32:64]).Uint64())
		}
	case "VaultCreated":
		if len(log.Topics) > 2 {
			parsed.Owner = common.BytesToAddress(log.Topics[2].Bytes()).Hex()
		}
	case "QuorumResultSubmitted":
		if len(log.Data) >= 32 {
			parsed.QuorumMet = new(big.Int).SetBytes(log.Data[:32]).Uint64() == 1
		}
	}

	return parsed, nil
}

// Close disconnects from the RPC node.
func (c *Client) Close() {
	c.eth.Close()
}
