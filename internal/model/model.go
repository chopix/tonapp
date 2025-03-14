package model

import (
	"time"
)

// Item represents a basic item in the system
type Item struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateItemRequest represents the request body for creating an item
type CreateItemRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// UpdateItemRequest represents the request body for updating an item
type UpdateItemRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type User struct {
	ID            int            `json:"id"`
	PubKey        string         `json:"pub_key"`
	Balance       float64        `json:"balance"`
	RefID         *int           `json:"ref_id,omitempty"`
	Investments   []Investment   `json:"investments,omitempty"`
	ReferralStats *ReferralStats `json:"referral_stats,omitempty"`
}

type Investment struct {
	ID        int     `json:"id"`
	UserID    int     `json:"user_id"`
	Type      string  `json:"type"`
	Amount    float64 `json:"amount"`
	CreatedAt int64   `json:"created_at"`
}

// ReferralStats represents referral statistics
type ReferralStats struct {
	TotalReferrals   int              `json:"total_referrals"`
	TotalEarnings    float64          `json:"total_earnings"`
	ReferralsByLevel []ReferralDetail `json:"referrals_by_level"`
}

// ReferralDetail represents detailed information about a referral
type ReferralDetail struct {
	UserID           int     `json:"user_id"`
	Level            int     `json:"level"`
	TotalInvested    float64 `json:"total_invested"`
	EarningsFromUser float64 `json:"earnings_from_user"`
	Level1Earnings   float64 `json:"level1_earnings"`
	Level2Earnings   float64 `json:"level2_earnings"`
	Level3Earnings   float64 `json:"level3_earnings"`
}

// ReferralEarning represents a single referral earning record
type ReferralEarning struct {
	ID          int64   `json:"id"`
	ReferrerID  int     `json:"referrer_id"`
	ReferredID  int     `json:"referred_id"`
	Amount      float64 `json:"amount"`
	Level       int     `json:"level"`
	CreatedAt   int64   `json:"created_at"`
}

type Referral struct {
	UserID         int     `json:"user_id"`
	TotalInvested  float64 `json:"total_invested"`
	EarningsFromUser float64 `json:"earnings_from_user"`
}

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type ReferralTier struct {
	MinReferrals int     `json:"min_referrals"`
	Percent      float64 `json:"percent"`
}

type InvestmentTypeConfig struct {
	WeeklyPercent float64 `json:"weekly_percent"`
	MinAmount     float64 `json:"min_amount"`
	LockPeriod    int     `json:"lock_period_days"` // 0 means can withdraw anytime
}

type TelegramConfig struct {
	BotToken    string `json:"bot_token"`
	WebAppURL   string `json:"web_app_url"`
	WelcomeText string `json:"welcome_text"`
	ButtonText  string `json:"button_text"`
}

type TONConfig struct {
	Network          string `json:"network"` // "mainnet" or "testnet"
	Mnemonic         string `json:"mnemonic"`
	APIKey           string `json:"api_key"`
	WalletVersion    string `json:"wallet_version"`
	FeeWalletAddress string `json:"fee_wallet_address"`
}

type DistributionWallet struct {
	FeeWallet  string `json:"fee_wallet"`
	MainWallet string `json:"main_wallet"`
}

type RateLimitConfig struct {
	RequestsPerSecond int `json:"requests_per_second"`
	BurstSize         int `json:"burst_size"` // Максимальное количество запросов в пике
}

type ReferralConfig struct {
	Level1Percent float64 `json:"level1_percent"` // 7% for direct referrals
	Level2Percent float64 `json:"level2_percent"` // 3% for second level
	Level3Percent float64 `json:"level3_percent"` // 1% for third level
}

// Configuration for investment types and their rules
type Config struct {
	InvestmentTypes map[string]InvestmentTypeConfig `json:"investment_types"`
	ReferralConfig  ReferralConfig                  `json:"referral_config"`
	AdminAPIKey     string                          `json:"admin_api_key"`
	Telegram        TelegramConfig                  `json:"telegram"`
	TON             TONConfig                       `json:"ton"`
	RateLimit       RateLimitConfig                 `json:"rate_limit"`
}

// OperationType represents the type of operation
type OperationType string

const (
	OperationTypeInvestmentCreated  OperationType = "investment_created"
	OperationTypeInvestmentClosed   OperationType = "investment_closed"
	OperationTypeDeposit           OperationType = "deposit"
	OperationTypeWithdrawal        OperationType = "withdrawal"
)

// Operation represents a user operation in the system
type Operation struct {
	ID          int64         `json:"id"`
	UserID      int           `json:"user_id"`
	Type        OperationType `json:"type"`
	Amount      float64       `json:"amount"`
	Description string        `json:"description"`
	CreatedAt   int64        `json:"created_at"`
	Status      string        `json:"status,omitempty"`
	Extra       interface{}   `json:"extra,omitempty"`
}

// OperationHistory represents a list of operations with pagination info
type OperationHistory struct {
	Operations []Operation `json:"operations"`
	Total      int        `json:"total"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
}
