package handler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"tonapp/internal/database"
	"tonapp/internal/model"
	"tonapp/internal/ton"

	"github.com/gin-gonic/gin"
)

// Handler manages HTTP request handling and business logic
type Handler struct {
	db     *database.Database
	config model.Config
	ton    *ton.Client
}

// NewHandler creates a new Handler instance with the given database and config
func NewHandler(db *database.Database, configPath string) (*Handler, error) {
	configFile, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config model.Config
	if err := json.Unmarshal(configFile, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	isTestnet := config.TON.Network == "testnet"
	tonClient := ton.NewClient(config.TON.APIKey, isTestnet, config.TON.Mnemonic, config.TON.WalletVersion, config.TON.FeeWalletAddress)

	return &Handler{
		db:     db,
		config: config,
		ton:    tonClient,
	}, nil
}

// AdminAuth middleware checks if the request has a valid admin API key
func (h *Handler) AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey != h.config.AdminAPIKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, model.Response{
				Success: false,
				Error:   "invalid API key",
			})
			return
		}
		c.Next()
	}
}

// CreateUser handles user creation requests
func (h *Handler) CreateUser(c *gin.Context) {
	var req struct {
		PubKey string  `json:"pub_key" binding:"required"`
		RefID  *int    `json:"ref_id"`
		ID     *int    `json:"id"`
		Name   *string `json:"name"`
		Photo  *string `json:"photo"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Success: false,
			Error:   "invalid request body",
		})
		return
	}

	user, err := h.db.CreateUser(req.PubKey, req.RefID, req.ID, req.Name, req.Photo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Success: false,
			Error:   fmt.Sprintf("failed to create user: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Success: true,
		Data:    user,
	})
}

// GetUser handles user retrieval requests
func (h *Handler) GetUser(c *gin.Context) {
	pubKey := c.Param("pub_key")
	if pubKey == "" {
		c.JSON(http.StatusBadRequest, model.Response{
			Success: false,
			Error:   "public key is required",
		})
		return
	}

	user, err := h.db.GetUserByPubKey(pubKey)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, model.Response{
			Success: false,
			Error:   "user not found",
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Success: false,
			Error:   "failed to get user",
		})
		return
	}
	c.JSON(http.StatusOK, model.Response{
		Success: true,
		Data:    user,
	})
}

// DeleteUser handles user deletion requests (admin only)
func (h *Handler) DeleteUser(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Success: false,
			Error:   "invalid user ID",
		})
		return
	}

	if err := h.db.DeleteUser(userID); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Success: false,
			Error:   "failed to delete user",
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Success: true,
		Data:    gin.H{"id": userID},
	})
}

// CreateInvestment handles investment creation requests
func (h *Handler) CreateInvestment(c *gin.Context) {
	pubKey := c.Param("pub_key")
	if pubKey == "" {
		c.JSON(http.StatusBadRequest, model.Response{
			Success: false,
			Error:   "public key is required",
		})
		return
	}

	var req struct {
		Type   string  `json:"type" binding:"required"`
		Amount float64 `json:"amount" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Success: false,
			Error:   "invalid request body",
		})
		return
	}

	investConfig, ok := h.config.InvestmentTypes[req.Type]
	if !ok {
		c.JSON(http.StatusBadRequest, model.Response{
			Success: false,
			Error:   "invalid investment type",
		})
		return
	}

	if req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, model.Response{
			Success: false,
			Error:   "investment amount must be positive",
		})
		return
	}

	user, err := h.db.GetUserByPubKey(pubKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Success: false,
			Error:   "failed to get user information",
		})
		return
	}

	if err := h.db.CreateInvestment(user.ID, req.Type, req.Amount, investConfig); err != nil {
		if err.Error() == "insufficient balance" {
			c.JSON(http.StatusBadRequest, model.Response{
				Success: false,
				Error:   fmt.Sprintf("insufficient balance: you have %.9f TON but need %.9f TON", user.Balance, req.Amount),
			})
			return
		}
		c.JSON(http.StatusBadRequest, model.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	lockPeriodText := "can withdraw anytime"
	if investConfig.LockPeriod > 0 {
		lockPeriodText = fmt.Sprintf("locked for %d days", investConfig.LockPeriod)
	}

	exampleProfit := req.Amount * (investConfig.WeeklyPercent / 100.0)

	c.JSON(http.StatusCreated, model.Response{
		Success: true,
		Data: gin.H{
			"message":               "investment created successfully",
			"amount":                req.Amount,
			"type":                  req.Type,
			"weekly_percent":        investConfig.WeeklyPercent,
			"example_weekly_profit": exampleProfit,
			"lock_period":           lockPeriodText,
			"remaining_balance":     user.Balance - req.Amount,
		},
	})
}

// DeleteInvestment handles investment deletion requests
func (h *Handler) DeleteInvestment(c *gin.Context) {
	pubKey := c.Param("pubkey")
	investmentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Success: false,
			Error:   "invalid investment id",
		})
		return
	}

	user, err := h.db.GetUserByPubKey(pubKey)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{
			Success: false,
			Error:   "user not found",
		})
		return
	}

	if err := h.db.DeleteInvestment(user.ID, investmentID); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Success: true,
		Data: gin.H{
			"message": "investment deleted successfully",
		},
	})
}

// GetReferralStats handles requests for referral statistics
func (h *Handler) GetReferralStats(c *gin.Context) {
	pubKey := c.Param("pub_key")
	if pubKey == "" {
		c.JSON(http.StatusBadRequest, model.Response{
			Success: false,
			Error:   "missing pub_key parameter",
		})
		return
	}

	stats, err := h.db.GetReferralStats(pubKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Success: false,
			Error:   fmt.Sprintf("failed to get referral stats: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Success: true,
		Data:    stats,
	})
}

// ProcessReferralEarnings processes referral earnings for an investment profit
func (h *Handler) ProcessReferralEarnings(userID int, profitAmount float64) error {
	// Get user's referrer chain (up to 3 levels)
	var referrerChain []int
	currentUserID := userID

	for i := 0; i < 3; i++ {
		var refID sql.NullInt64
		err := h.db.DB().QueryRow("SELECT ref_id FROM users WHERE id = ?", currentUserID).Scan(&refID)
		if err != nil {
			return err
		}
		if !refID.Valid {
			break
		}
		referrerChain = append(referrerChain, int(refID.Int64))
		currentUserID = int(refID.Int64)
	}

	// Calculate and add earnings for each level
	for level, referrerID := range referrerChain {
		level++ // Convert to 1-based level number
		var percent float64
		switch level {
		case 1:
			percent = h.config.ReferralConfig.Level1Percent
		case 2:
			percent = h.config.ReferralConfig.Level2Percent
		case 3:
			percent = h.config.ReferralConfig.Level3Percent
		}

		earnings := profitAmount * (percent / 100.0)
		if err := h.db.AddReferralEarning(referrerID, userID, earnings, level); err != nil {
			return err
		}
	}

	return nil
}

// UpdateUserBalance handles user balance updates (admin only)
func (h *Handler) UpdateUserBalance(c *gin.Context) {
	var req struct {
		UserID  int     `json:"user_id" binding:"required"`
		Balance float64 `json:"balance" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Success: false,
			Error:   "invalid request body",
		})
		return
	}

	if err := h.db.UpdateUserBalance(req.UserID, req.Balance); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Success: false,
			Error:   fmt.Sprintf("failed to update balance: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Success: true,
		Data: map[string]interface{}{
			"user_id": req.UserID,
			"balance": req.Balance,
		},
	})
}

// GetConfigPublic returns the current configuration without admin API key and Ton config
func (h *Handler) GetConfigPublic() model.ConfigPublic {
	config := h.config
	return model.ConfigPublic{
		InvestmentTypes: config.InvestmentTypes,
		ReferralConfig:  config.ReferralConfig,
	}
}

// GetConfig returns the current configuration
func (h *Handler) GetConfig() model.Config {
	return h.config
}

// CreateDeposit handles deposit creation requests
func (h *Handler) CreateDeposit(c *gin.Context) {
	var req model.CreateDepositRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Success: false,
			Error:   "invalid request body",
		})
		return
	}

	user, err := h.db.GetUserByPubKey(req.PubKey)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{
			Success: false,
			Error:   "user not found",
		})
		return
	}

	walletAddress := h.ton.GetDepositAddress()
	if walletAddress == "" {
		c.JSON(http.StatusInternalServerError, model.Response{
			Success: false,
			Error:   "failed to get deposit wallet address",
		})
		return
	}

	memo := fmt.Sprintf("TON%d%d", user.ID, time.Now().Unix())

	deposit, err := h.db.CreateDepositRequest(user.ID, req.Amount, memo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Success: false,
			Error:   "failed to create deposit request",
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Success: true,
		Data: model.DepositResponse{
			ID:            deposit.ID,
			Amount:        deposit.Amount,
			Status:        deposit.Status,
			Memo:          deposit.Memo,
			WalletAddress: walletAddress,
		},
	})
}

// ConfirmDeposit handles deposit confirmation requests
func (h *Handler) ConfirmDeposit(c *gin.Context) {
	var req model.ConfirmDepositRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Success: false,
			Error:   "invalid request body",
		})
		return
	}

	user, err := h.db.GetUserByPubKey(req.PubKey)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{
			Success: false,
			Error:   "user not found",
		})
		return
	}

	deposit, err := h.db.GetDepositRequest(req.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{
			Success: false,
			Error:   "deposit request not found",
		})
		return
	}

	if deposit.UserID != user.ID {
		c.JSON(http.StatusForbidden, model.Response{
			Success: false,
			Error:   "deposit request does not belong to user",
		})
		return
	}

	if deposit.Status != "pending" {
		c.JSON(http.StatusBadRequest, model.Response{
			Success: false,
			Error:   "deposit request is not pending",
		})
		return
	}

	walletAddress := h.ton.GetDepositAddress()
	if walletAddress == "" {
		c.JSON(http.StatusInternalServerError, model.Response{
			Success: false,
			Error:   "failed to get deposit wallet address",
		})
		return
	}

	fmt.Printf("Checking deposit for wallet %s, amount %.9f TON, memo %s\n",
		walletAddress, deposit.Amount, deposit.Memo)

	received, err := h.ton.CheckDeposit(walletAddress, deposit.Amount, deposit.Memo, 30)
	if err != nil {
		fmt.Printf("Failed to check transaction: %v\n", err)
		c.JSON(http.StatusInternalServerError, model.Response{
			Success: false,
			Error:   "failed to check transaction",
		})
		return
	}

	if !received {
		c.JSON(http.StatusBadRequest, model.Response{
			Success: false,
			Error:   "payment not received",
		})
		return
	}

	if err := h.db.UpdateDepositStatus(deposit.ID, "completed"); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Success: false,
			Error:   "failed to update deposit status",
		})
		return
	}

	if err := h.db.UpdateUserBalance(user.ID, user.Balance+deposit.Amount); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Success: false,
			Error:   "failed to update user balance",
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Success: true,
		Data: gin.H{
			"status": "completed",
		},
	})
}

// WithdrawFunds handles withdrawal requests
func (h *Handler) WithdrawFunds(c *gin.Context) {
	var req model.WithdrawalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Success: false,
			Error:   "invalid request body",
		})
		return
	}

	user, err := h.db.GetUserByPubKey(req.PubKey)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{
			Success: false,
			Error:   "user not found",
		})
		return
	}

	deposits, err := h.db.GetDepositsOfUser(user.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{
			Success: false,
			Error:   "user not found error",
		})
		return
	}

	MathDeposits := 0.0
	for _, deposit := range deposits {
		if deposit.Status == "completed" {
			MathDeposits += deposit.Amount
		} else {
			c.JSON(http.StatusBadRequest, model.Response{
				Success: false,
				Error:   "user has uncompleted deposits",
			})
			return
		}
	}

	withdrawals, err := h.db.GetWithdrawalRequestsByUser(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Success: false,
			Error:   "failed to get withdrawal history",
		})
		return
	}

	Mathwithdrawal := 0.0
	for _, withdrawal := range withdrawals {
		if withdrawal.Status == "completed" {
			Mathwithdrawal += withdrawal.Amount
		} else {
			c.JSON(http.StatusBadRequest, model.Response{
				Success: false,
				Error:   "user has uncompleted withdrawals",
			})
			return
		}
	}

	availableBalance := MathDeposits
	availableBalance -= MathDeposits * 0.2 // Apply 20% fee
	availableBalance -= Mathwithdrawal     // Subtract previous withdrawals

	if availableBalance < req.Amount {
		c.JSON(http.StatusBadRequest, model.Response{
			Success: false,
			Error:   fmt.Sprintf("insufficient balance: have %.2f TON, requested %.2f TON", availableBalance, req.Amount),
		})
		return
	}

	if user.Balance < req.Amount {
		c.JSON(http.StatusBadRequest, model.Response{
			Success: false,
			Error:   fmt.Sprintf("insufficient balance: have %.2f TON, requested %.2f TON", user.Balance, req.Amount),
		})
		return
	}

	_, err = h.db.CreateWithdrawalRequest(user.ID, req.Amount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Success: false,
			Error:   fmt.Sprintf("Failed to create withdrawal request in database"),
		})
		return
	}
	_, err = h.db.ConfirmWithdrawalRequest(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Success: false,
			Error:   fmt.Sprintf("failed to confirm withdrawal"),
		})
		return
	}

	// Withdraw funds and get transaction hash
	txHash, err := h.ton.WithdrawUserFunds(c.Request.Context(), req.PubKey, req.Amount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Success: false,
			Error:   fmt.Sprintf("Failed to withdraw funds: %v", err),
		})
		fmt.Printf("Failed to withdraw funds: %v\n", err)
		return
	}

	// Store transaction hash
	err = h.db.UpdateWithdrawalTxHash(user.ID, txHash)
	if err != nil {
		fmt.Printf("Failed to store transaction hash: %v\n", err)
		// Don't return error to user since the withdrawal was successful
	}

	newBalance := user.Balance - req.Amount
	err = h.db.UpdateUserBalance(user.ID, newBalance)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Success: false,
			Error:   fmt.Sprintf("Failed to update balance: %v", err),
		})
		return
	}

	userAddress, err := h.ton.GenerateWalletAddressFromPubKey(req.PubKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Success: false,
			Error:   fmt.Sprintf("Failed to generate wallet address: %v", err),
		})
		return
	}

	// Add operation record
	op := &model.Operation{
		UserID:      user.ID,
		Type:        "withdrawal",
		Amount:      req.Amount,
		Description: fmt.Sprintf("Withdrawal of %.2f TON", req.Amount),
		Extra:       fmt.Sprintf(`{"tx_hash":"%s"}`, txHash),
	}
	if err := h.db.AddOperation(op); err != nil {
		fmt.Printf("Failed to add operation record: %v\n", err)
		// Don't return error to user since the withdrawal was successful
	}

	c.JSON(http.StatusOK, model.WithdrawalResponse{
		Success: true,
		Amount:  req.Amount,
		Address: userAddress,
		TxHash:  txHash,
	})
}

// GetUserOperations handles requests for user operation history
func (h *Handler) GetUserOperations(c *gin.Context) {
	pubKey := c.Param("pub_key")
	if pubKey == "" {
		c.JSON(http.StatusBadRequest, model.Response{
			Success: false,
			Error:   "missing pub_key parameter",
		})
		return
	}

	// Get user by public key
	user, err := h.db.GetUserByPubKey(pubKey)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{
			Success: false,
			Error:   "user not found",
		})
		return
	}

	// Get page and page_size from query parameters
	page := 1
	pageSize := 10

	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := c.Query("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	// Get operations
	history, err := h.db.GetUserOperations(user.ID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{
			Success: false,
			Error:   fmt.Sprintf("failed to get operations: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Success: true,
		Data:    history,
	})
}
