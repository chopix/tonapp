package ton

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton"
	"github.com/xssnick/tonutils-go/ton/wallet"
)

type Client struct {
	apiKey           string
	baseURL          string
	isTestnet        bool
	seedPhrase       string
	address          string
	walletType       wallet.Version
	feeWalletAddress string
}

func NewClient(apiKey string, isTestnet bool, seedPhrase string, walletVersion string, feeWalletAddress string) *Client {
	var baseURL string
	baseURL = "https://toncenter.com/api/v2"
	if isTestnet {
		baseURL = "https://testnet.toncenter.com/api/v2"
	}

	// Parse wallet version
	version := wallet.V4R2
	switch walletVersion {
	case "V3R1":
		version = wallet.V3R1
	case "V3R2":
		version = wallet.V3R2
	case "V4R1":
		version = wallet.V4R1
	case "V4R2":
		version = wallet.V4R2
	case "HighloadV2R2":
		version = wallet.HighloadV2R2
	}

	c := &Client{
		apiKey:           apiKey,
		baseURL:          baseURL,
		isTestnet:        isTestnet,
		seedPhrase:       seedPhrase,
		walletType:       version,
		feeWalletAddress: feeWalletAddress,
	}

	// Generate wallet address from seed phrase
	addr, err := c.generateWalletAddress()
	if err != nil {
		// Log error but don't fail - we'll try to generate address again when needed
		fmt.Printf("Failed to generate initial wallet address: %v\n", err)
	} else {
		c.address = addr
	}

	return c
}

type Wallet struct {
	PrivateKey string
	PublicKey  string
	Address    string
}

func (c *Client) generateWalletAddress() (string, error) {
	if c.address != "" {
		return c.address, nil
	}

	// Split seed phrase into words
	words := strings.Split(c.seedPhrase, " ")

	// Initialize TON connection
	client := liteclient.NewConnectionPool()
	configUrl := "https://ton.org/global.config.json"
	if c.isTestnet {
		configUrl = "https://ton-blockchain.github.io/testnet-global.config.json"
	}
	err := client.AddConnectionsFromConfigUrl(context.Background(), configUrl)
	if err != nil {
		panic(err)
	}
	api := ton.NewAPIClient(client)

	// Create wallet from seed phrase using specified version
	w, err := wallet.FromSeed(api, words, c.walletType)
	if err != nil {
		return "", fmt.Errorf("failed to create wallet from seed phrase: %v", err)
	}

	// Get wallet address
	addr := w.WalletAddress()
	if addr == nil {
		return "", fmt.Errorf("failed to get wallet address")
	}

	return addr.String(), nil
}

func (c *Client) GetDepositAddress() string {
	if c.address == "" {
		addr, err := c.generateWalletAddress()
		if err != nil {
			fmt.Printf("Failed to generate wallet address: %v\n", err)
			return ""
		}
		c.address = addr
	}
	return c.address
}

type Message struct {
	Value   string `json:"value"`
	Message string `json:"message"`
}

type Transaction struct {
	Utime int64   `json:"utime"`
	InMsg Message `json:"in_msg"`
}

type TransactionsResponse struct {
	OK     bool          `json:"ok"`
	Result []Transaction `json:"result"`
}
type BalanceResponse struct {
	OK     bool   `json:"ok"`
	Result string `json:"result"`
}

// CheckDeposit verifies if a deposit transaction exists
func (c *Client) CheckDeposit(walletAddress string, expectedAmount float64, memo string, withinLastMinutes int) (bool, error) {

	// Build URL with parameters
	endpoint := fmt.Sprintf("%s/getTransactions", c.baseURL)
	params := url.Values{
		"address":  {walletAddress},
		"limit":    {"50"},
		"archival": {"true"},
	}

	reqURL := fmt.Sprintf("%s?%s", endpoint, params.Encode())
	fmt.Printf("Checking transactions at URL: %s\n", reqURL)

	// Create request
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %v", err)
	}

	// Add API key
	req.Header.Set("X-API-Key", c.apiKey)

	// Make request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response: %v", err)
	}

	fmt.Printf("Response from TON Center: %s\n", string(body))

	// Parse response
	var result TransactionsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return false, fmt.Errorf("failed to parse response: %v", err)
	}

	if !result.OK {
		return false, fmt.Errorf("API returned not OK status")
	}

	// Calculate time threshold
	threshold := time.Now().Add(-time.Duration(withinLastMinutes) * time.Minute).Unix()
	fmt.Printf("Looking for transactions after: %v with memo: %s\n",
		time.Unix(threshold, 0), memo)

	// Check transactions
	for _, tx := range result.Result {
		fmt.Printf("Found transaction at %v with amount %s and memo: %s\n",
			time.Unix(tx.Utime, 0), tx.InMsg.Value, tx.InMsg.Message)

		// Skip if transaction is too old
		if tx.Utime < threshold {
			continue
		}

		// Skip if memo doesn't match
		if tx.InMsg.Message != memo {
			continue
		}

		// Parse amount in nanotons
		amountNano, err := strconv.ParseInt(tx.InMsg.Value, 10, 64)
		if err != nil {
			fmt.Printf("Failed to parse amount: %v\n", err)
			continue // Skip if amount cannot be parsed
		}

		amountTON := fromNano(amountNano)
		fmt.Printf("Transaction amount in TON: %v, expected: %v\n", amountTON, expectedAmount)

		// Compare amounts in TON with small epsilon for float comparison
		if math.Abs(amountTON-expectedAmount) < 0.000001 {
			err := c.TransferFundsWithSplit(context.Background(), amountTON, c.feeWalletAddress)
			if err != nil {
				return false, err
			}
			return true, nil
		}
	}

	return false, nil
}
func (c *Client) GetMainWalletAddress() (string, error) {
	client := liteclient.NewConnectionPool()
	configUrl := "https://ton.org/global.config.json"
	if c.isTestnet {
		configUrl = "https://ton-blockchain.github.io/testnet-global.config.json"
	}

	err := client.AddConnectionsFromConfigUrl(context.Background(), configUrl)
	if err != nil {
		return "", fmt.Errorf("failed to connect to TON: %v", err)
	}

	api := ton.NewAPIClient(client)

	// Create wallet instance from seed phrase
	words := strings.Split(c.seedPhrase, " ")
	w, err := wallet.FromSeed(api, words, c.walletType)
	if err != nil {
		return "", fmt.Errorf("failed to create wallet from seed: %v", err)
	}
	return w.Address().String(), nil
}

// TransferFundsWithSplit transfers TON from the main wallet to fee addresse with 20% split
func (c *Client) TransferFundsWithSplit(ctx context.Context, amount float64, feeAddress string) error {
	// Initialize connection
	client := liteclient.NewConnectionPool()
	configUrl := "https://ton.org/global.config.json"
	if c.isTestnet {
		configUrl = "https://ton-blockchain.github.io/testnet-global.config.json"
	}

	err := client.AddConnectionsFromConfigUrl(ctx, configUrl)
	if err != nil {
		return fmt.Errorf("failed to connect to TON: %v", err)
	}

	api := ton.NewAPIClient(client)

	// Create wallet instance from seed phrase
	words := strings.Split(c.seedPhrase, " ")
	w, err := wallet.FromSeed(api, words, c.walletType)
	if err != nil {
		return fmt.Errorf("failed to create wallet from seed: %v", err)
	}

	feeAmount := amount * 0.2 // 20%

	feeNano := toNano(feeAmount)

	addr := address.MustParseAddr(feeAddress)
	err = w.Transfer(context.Background(), addr, tlb.MustFromNano(big.NewInt(feeNano), 0), "")

	if err != nil {
		return fmt.Errorf("failed to send transfers: %v", err)
	}

	return nil
}

// Helper function to convert TON amount to nanotons
func toNano(tons float64) int64 {
	return int64(tons * 1000000000) // 1 TON = 10^9 nanotons
}

// Helper function to convert nanotons to TON
func fromNano(nanotons int64) float64 {
	return float64(nanotons) / 1000000000 // 1 TON = 10^9 nanotons
}

// GetWalletBalance returns the balance of a wallet in TON
func (c *Client) GetWalletBalance(ctx context.Context, addr string) (float64, error) {
	endpoint := fmt.Sprintf("%s/getAddressBalance", c.baseURL)
	params := url.Values{
		"address": {addr},
	}

	reqURL := fmt.Sprintf("%s?%s", endpoint, params.Encode())
	fmt.Printf("Checking balance at URL: %s\n", reqURL)

	// Create request
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %v", err)
	}

	// Add API key
	req.Header.Set("X-API-Key", c.apiKey)

	// Make request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response: %v", err)
	}

	// Parse response
	var result BalanceResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("failed to parse response: %v", err)
	}

	if !result.OK {
		return 0, fmt.Errorf("API returned not OK status")
	}

	// Convert balance string to int64
	balanceNano, err := strconv.ParseInt(result.Result, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse balance: %v", err)
	}

	// Convert from nanotons to TON
	balance := fromNano(balanceNano)
	return balance, nil
}

// WithdrawUserFunds transfers TON from main wallet to user's wallet with validations
func (c *Client) WithdrawUserFunds(ctx context.Context, pubKey string, amount float64) (string, error) {
	// Get user's wallet address
	userAddress, err := c.GenerateWalletAddressFromPubKey(pubKey)
	if err != nil {
		return "", fmt.Errorf("failed to generate user wallet address: %v", err)
	}

	// Get main wallet
	w, err := c.getMainWallet(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get main wallet: %v", err)
	}

	// Check if main wallet has enough balance
	mainBalance, err := c.GetWalletBalance(ctx, c.address)
	if err != nil {
		return "", fmt.Errorf("failed to get main wallet balance: %v", err)
	}

	if mainBalance < amount {
		return "", fmt.Errorf("insufficient balance in main wallet")
	}

	// Convert amount to nanotons
	amountNano := toNano(amount)

	// Send transaction
	addr := address.MustParseAddr(userAddress)
	message, err := w.BuildTransfer(addr, tlb.MustFromNano(big.NewInt(amountNano), 0), false, "")
	if err != nil {
		return "", fmt.Errorf("failed to build transfer message: %v", err)
	}
	messages := []*wallet.Message{message}
	// Send transaction
	tx, err := w.SendManyWaitTxHash(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("failed to send withdrawal: %v", err)
	}

	return hex.EncodeToString(tx), nil
}

// GenerateWalletAddressFromPubKey generates TON wallet address from public key
func (c *Client) GenerateWalletAddressFromPubKey(pubKey string) (string, error) {
	// Decode hex string to bytes
	pubKeyBytes, err := hex.DecodeString(pubKey)
	if err != nil {
		return "", fmt.Errorf("failed to decode public key: %v", err)
	}

	// Convert to ed25519.PublicKey
	publicKey := ed25519.PublicKey(pubKeyBytes)

	// Generate address
	addr, err := wallet.AddressFromPubKey(publicKey, c.walletType, wallet.DefaultSubwallet)
	if err != nil {
		return "", fmt.Errorf("failed to create wallet from public key: %v", err)
	}

	return addr.String(), nil
}

func (c *Client) getMainWallet(ctx context.Context) (*wallet.Wallet, error) {
	// Initialize connection
	client := liteclient.NewConnectionPool()
	configUrl := "https://ton.org/global.config.json"
	if c.isTestnet {
		configUrl = "https://ton-blockchain.github.io/testnet-global.config.json"
	}

	err := client.AddConnectionsFromConfigUrl(ctx, configUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to TON: %v", err)
	}

	api := ton.NewAPIClient(client)

	// Create wallet instance from seed phrase
	words := strings.Split(c.seedPhrase, " ")
	w, err := wallet.FromSeed(api, words, c.walletType)
	if err != nil {
		return nil, fmt.Errorf("failed to create wallet from seed: %v", err)
	}

	return w, nil
}
