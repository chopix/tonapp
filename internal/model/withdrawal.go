package model

import "time"

// WithdrawalRequest represents the request body for withdrawing TON
type WithdrawalRequest struct {
	PubKey string  `json:"pub_key" binding:"required"`
	Amount float64 `json:"amount" binding:"required,gt=0"`
}

// WithdrawalResponse represents the response for a withdrawal request
type WithdrawalResponse struct {
	Success   bool    `json:"success"`
	Error     string  `json:"error,omitempty"`
	Amount    float64 `json:"amount,omitempty"`
	Address   string  `json:"address,omitempty"`
	TxHash    string  `json:"tx_hash,omitempty"`
}

type WithdrawalStorage struct {
	ID            int       `json:"id"`
	UserID        int       `json:"user_id"`
	Amount        float64   `json:"amount"`
	Status        string    `json:"status"` // pending, completed, failed
	CreatedAt     time.Time `json:"created_at"`
	TxHash        string    `json:"tx_hash,omitempty"`
}
