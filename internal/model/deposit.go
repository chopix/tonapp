package model

import "time"

type DepositRequest struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Amount    float64   `json:"amount"`
	Status    string    `json:"status"` // pending, completed, failed
	Memo      string    `json:"memo"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type DepositResponse struct {
	ID            int     `json:"id"`
	Amount        float64 `json:"amount"`
	Status        string  `json:"status"`
	Memo          string  `json:"memo"`
	WalletAddress string  `json:"wallet_address"`
}

type CreateDepositRequest struct {
	PubKey string  `json:"pub_key" binding:"required"`
	Amount float64 `json:"amount" binding:"required,min=1"`
}

type ConfirmDepositRequest struct {
	PubKey string `json:"pub_key" binding:"required"`
	ID     int    `json:"deposit_id" binding:"required"`
}
