package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"
	"tonapp/internal/model"

	_ "github.com/mattn/go-sqlite3"
)

const (
	// Transaction statuses
	StatusPending   = "pending"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// Database represents a connection to the SQLite database
type Database struct {
	db *sql.DB
}

// New creates a new Database instance and initializes the schema
func New(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %v", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error connecting to the database: %v", err)
	}

	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("error creating tables: %v", err)
	}

	return &Database{db: db}, nil
}

func createTables(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY,
			pub_key TEXT UNIQUE NOT NULL,
			balance REAL NOT NULL DEFAULT 0,
			ref_id INTEGER,
			FOREIGN KEY (ref_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS investments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			type TEXT NOT NULL,
			amount REAL NOT NULL,
			created_at INTEGER NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS referral_earnings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			referrer_id INTEGER NOT NULL,
			referred_id INTEGER NOT NULL,
			amount REAL NOT NULL,
			level INTEGER NOT NULL DEFAULT 1,
			created_at INTEGER NOT NULL,
			FOREIGN KEY (referrer_id) REFERENCES users(id),
			FOREIGN KEY (referred_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS deposit_requests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			amount REAL NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			memo TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS withdrawal_requests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			amount REAL NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at INTEGER NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS operations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			type TEXT NOT NULL,
			amount REAL NOT NULL,
			description TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			extra TEXT,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS withdrawals (
			id INTEGER PRIMARY KEY,
			user_id INTEGER NOT NULL,
			amount REAL NOT NULL,
			status TEXT NOT NULL,
			tx_hash TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("error executing query: %v\nQuery: %s", err, query)
		}
	}

	return nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

// CreateUser creates a new user with the given public key and optional parameters.
// If customID is provided, it will be used as the user's ID.
// If customID is nil, a random ID between 1000000000 and 1000000000000 will be generated.
// If refID is provided, it will be used to establish a referral relationship.
func (d *Database) CreateUser(pubKey string, refID *int, customID *int) (*model.User, error) {
	// Сначала проверяем, существует ли пользователь
	existingUser, err := d.GetUserByPubKey(pubKey)
	if err != sql.ErrNoRows && err != nil {
		return nil, err
	}
	if existingUser != nil {
		return existingUser, nil
	}

	tx, err := d.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Generate random ID if not provided
	var id int
	if customID != nil {
		id = *customID
	} else {
		// Generate random ID between 1000000000 and 1000000000000
		id = rand.Intn(1000000000000-1000000000) + 1000000000
	}

	stmt, err := tx.Prepare("INSERT INTO users (id, pub_key, balance, ref_id) VALUES (?, ?, ?, ?)")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	_, err = stmt.Exec(id, pubKey, 0, refID)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return d.GetUser(id)
}

// GetUserByPubKey retrieves a user by their public key
func (d *Database) GetUserByPubKey(pubKey string) (*model.User, error) {
	var user model.User
	var refID sql.NullInt64

	stmt, err := d.db.Prepare("SELECT id, pub_key, balance, ref_id FROM users WHERE pub_key = ?")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	err = stmt.QueryRow(pubKey).Scan(&user.ID, &user.PubKey, &user.Balance, &refID)

	if err == sql.ErrNoRows {
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	if refID.Valid {
		refIDInt := int(refID.Int64)
		user.RefID = &refIDInt
	}

	investments, err := d.getUserInvestments(user.ID)
	if err != nil {
		return nil, err
	}
	user.Investments = investments

	return &user, nil
}

// GetUser retrieves a user by their ID
func (d *Database) GetUser(id int) (*model.User, error) {
	var user model.User
	var refID sql.NullInt64

	stmt, err := d.db.Prepare("SELECT id, pub_key, balance, ref_id FROM users WHERE id = ?")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	err = stmt.QueryRow(id).Scan(&user.ID, &user.PubKey, &user.Balance, &refID)

	if err == sql.ErrNoRows {
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	if refID.Valid {
		refIDInt := int(refID.Int64)
		user.RefID = &refIDInt
	}

	investments, err := d.getUserInvestments(user.ID)
	if err != nil {
		return nil, err
	}
	user.Investments = investments

	return &user, nil
}

func (d *Database) DeleteUser(id int) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete user's investments first
	stmt, err := tx.Prepare("DELETE FROM investments WHERE user_id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	if _, err := stmt.Exec(id); err != nil {
		return err
	}

	// Delete user
	stmt, err = tx.Prepare("DELETE FROM users WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	if _, err := stmt.Exec(id); err != nil {
		return err
	}

	return tx.Commit()
}

func (d *Database) CreateInvestment(userID int, investType string, amount float64, config model.InvestmentTypeConfig) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check if user has enough balance
	var currentBalance float64
	err = tx.QueryRow("SELECT balance FROM users WHERE id = ?", userID).Scan(&currentBalance)
	if err != nil {
		return err
	}

	if currentBalance < amount {
		return fmt.Errorf("insufficient balance")
	}

	// Update user balance
	stmt, err := tx.Prepare("UPDATE users SET balance = balance - ? WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(amount, userID)
	if err != nil {
		return err
	}

	// Create investment
	stmt, err = tx.Prepare("INSERT INTO investments (user_id, type, amount, created_at) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().Unix()
	_, err = stmt.Exec(userID, investType, amount, now)
	if err != nil {
		return err
	}

	// Add operation
	op := &model.Operation{
		UserID:      userID,
		Type:        model.OperationTypeInvestmentCreated,
		Amount:      amount,
		Description: fmt.Sprintf("Created %s investment", investType),
		CreatedAt:   now,
		Extra: map[string]interface{}{
			"type":           investType,
			"weekly_percent": config.WeeklyPercent,
			"lock_period":    config.LockPeriod,
		},
	}

	stmt, err = tx.Prepare(`
		INSERT INTO operations (user_id, type, amount, description, created_at, extra)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	extraJSON, err := json.Marshal(op.Extra)
	if err != nil {
		return err
	}

	_, err = stmt.Exec(
		op.UserID,
		op.Type,
		op.Amount,
		op.Description,
		op.CreatedAt,
		extraJSON,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (d *Database) DeleteInvestment(userID int, investmentID int64) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get investment details
	var investment struct {
		Amount    float64
		Type      string
		CreatedAt int64
	}
	err = tx.QueryRow(`
		SELECT amount, type, created_at 
		FROM investments 
		WHERE id = ? AND user_id = ?`,
		investmentID, userID).Scan(&investment.Amount, &investment.Type, &investment.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("investment not found")
		}
		return err
	}

	// Delete investment
	stmt, err := tx.Prepare("DELETE FROM investments WHERE id = ? AND user_id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	result, err := stmt.Exec(investmentID, userID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("investment not found")
	}

	// Return funds to user
	stmt, err = tx.Prepare("UPDATE users SET balance = balance + ? WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(investment.Amount, userID)
	if err != nil {
		return err
	}

	// Add operation
	now := time.Now().Unix()
	op := &model.Operation{
		UserID:      userID,
		Type:        model.OperationTypeInvestmentClosed,
		Amount:      investment.Amount,
		Description: fmt.Sprintf("Closed %s investment", investment.Type),
		CreatedAt:   now,
		Extra: map[string]interface{}{
			"type":               investment.Type,
			"investment_id":      investmentID,
			"investment_created": investment.CreatedAt,
			"duration_days":      (now - investment.CreatedAt) / 86400, // Convert seconds to days
		},
	}

	stmt, err = tx.Prepare(`
		INSERT INTO operations (user_id, type, amount, description, created_at, extra)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	extraJSON, err := json.Marshal(op.Extra)
	if err != nil {
		return err
	}

	_, err = stmt.Exec(
		op.UserID,
		op.Type,
		op.Amount,
		op.Description,
		op.CreatedAt,
		extraJSON,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// Get USD rate from external API https://api.coingecko.com/api/v3/coins/the-open-network
func (d *Database) GetUsdRate() float64 {
	resp, err := http.Get("https://api.coingecko.com/api/v3/coins/the-open-network")
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0
	}

	rate, ok := data["market_data"].(map[string]interface{})["current_price"].(map[string]interface{})["usd"].(float64)
	if !ok {
		return 0
	}

	return rate
}

func (d *Database) getUserInvestments(userID int) ([]model.Investment, error) {
	stmt, err := d.db.Prepare("SELECT id, user_id, type, amount, created_at FROM investments WHERE user_id = ?")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var investments []model.Investment
	for rows.Next() {
		var inv model.Investment
		if err := rows.Scan(&inv.ID, &inv.UserID, &inv.Type, &inv.Amount, &inv.CreatedAt); err != nil {
			return nil, err
		}
		investments = append(investments, inv)
	}

	return investments, nil
}

func (d *Database) GetReferralStats(pubKey string) (*model.ReferralStats, error) {
	// Get user by public key
	user, err := d.GetUserByPubKey(pubKey)
	if err != nil {
		return nil, err
	}

	// Get total earnings from referral_earnings table
	var totalEarnings float64
	err = d.db.QueryRow(`
		SELECT COALESCE(SUM(amount), 0)
		FROM referral_earnings
		WHERE referrer_id = ?`,
		user.ID).Scan(&totalEarnings)
	if err != nil {
		return nil, err
	}

	// Get referrals by level
	var referralsByLevel []model.ReferralDetail

	// Get level 1 referrals (direct)
	level1Referrals, err := d.getLevelReferrals(user.ID, 1)
	if err != nil {
		return nil, err
	}

	// Get level 2 referrals (referrals of referrals)
	level2Referrals, err := d.getLevelReferrals(user.ID, 2)
	if err != nil {
		return nil, err
	}

	// Get level 3 referrals
	level3Referrals, err := d.getLevelReferrals(user.ID, 3)
	if err != nil {
		return nil, err
	}

	// Combine all referrals
	allReferrals := make(map[int]*model.ReferralDetail)

	// Process level 1 referrals
	for _, ref := range level1Referrals {
		detail := &model.ReferralDetail{
			UserID:           ref.UserID,
			Level:            1,
			TotalInvested:    ref.TotalInvested,
			EarningsFromUser: ref.EarningsFromUser,
		}
		allReferrals[ref.UserID] = detail
	}

	// Process level 2 referrals
	for _, ref := range level2Referrals {
		if detail, exists := allReferrals[ref.UserID]; exists {
			detail.Level2Earnings = ref.EarningsFromUser
		} else {
			detail := &model.ReferralDetail{
				UserID:           ref.UserID,
				Level:            2,
				TotalInvested:    ref.TotalInvested,
				EarningsFromUser: ref.EarningsFromUser,
			}
			allReferrals[ref.UserID] = detail
		}
	}

	// Process level 3 referrals
	for _, ref := range level3Referrals {
		if detail, exists := allReferrals[ref.UserID]; exists {
			detail.Level3Earnings = ref.EarningsFromUser
		} else {
			detail := &model.ReferralDetail{
				UserID:           ref.UserID,
				Level:            3,
				TotalInvested:    ref.TotalInvested,
				EarningsFromUser: ref.EarningsFromUser,
			}
			allReferrals[ref.UserID] = detail
		}
	}

	// Convert map to slice
	for _, detail := range allReferrals {
		referralsByLevel = append(referralsByLevel, *detail)
	}

	return &model.ReferralStats{
		TotalReferrals:   len(allReferrals),
		TotalEarnings:    totalEarnings,
		ReferralsByLevel: referralsByLevel,
	}, nil
}

func (d *Database) getLevelReferrals(userID int, level int) ([]model.Referral, error) {
	var refs []model.Referral

	// For level 1, get direct referrals
	// For level 2, get referrals of referrals
	// For level 3, get referrals of level 2 referrals
	var query string
	switch level {
	case 1:
		query = `SELECT id FROM users WHERE ref_id = ?`
	case 2:
		query = `SELECT u2.id 
				FROM users u1 
				JOIN users u2 ON u2.ref_id = u1.id 
				WHERE u1.ref_id = ?`
	case 3:
		query = `SELECT u3.id 
				FROM users u1 
				JOIN users u2 ON u2.ref_id = u1.id 
				JOIN users u3 ON u3.ref_id = u2.id 
				WHERE u1.ref_id = ?`
	default:
		return nil, fmt.Errorf("invalid referral level")
	}

	rows, err := d.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var refID int
		if err := rows.Scan(&refID); err != nil {
			return nil, err
		}

		// Get total invested amount
		var totalInvested float64
		err = d.db.QueryRow(`
			SELECT COALESCE(SUM(amount), 0) 
			FROM investments 
			WHERE user_id = ?`, refID).Scan(&totalInvested)
		if err != nil {
			return nil, err
		}

		// Get earnings from this referral
		var earningsFromUser float64
		err = d.db.QueryRow(`
			SELECT COALESCE(SUM(amount), 0) 
			FROM referral_earnings 
			WHERE referrer_id = ? AND referred_id = ?`,
			userID, refID).Scan(&earningsFromUser)
		if err != nil {
			return nil, err
		}

		refs = append(refs, model.Referral{
			UserID:           refID,
			TotalInvested:    totalInvested,
			EarningsFromUser: earningsFromUser,
		})
	}

	return refs, nil
}

func (d *Database) AddReferralEarning(referrerID int, referredID int, amount float64, level int) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Add referral earning record
	_, err = tx.Exec(`
		INSERT INTO referral_earnings (referrer_id, referred_id, amount, level, created_at) 
		VALUES (?, ?, ?, ?, ?)`,
		referrerID, referredID, amount, level, time.Now().Unix())
	if err != nil {
		return err
	}

	// Update referrer's balance
	_, err = tx.Exec("UPDATE users SET balance = balance + ? WHERE id = ?",
		amount, referrerID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// UpdateUserBalance updates the balance of a user by their ID
func (d *Database) UpdateUserBalance(userID int, newBalance float64) error {
	stmt, err := d.db.Prepare("UPDATE users SET balance = ? WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(newBalance, userID)
	return err
}

// CreateDepositRequest creates a new deposit request
func (d *Database) CreateDepositRequest(userID int, amount float64, memo string) (*model.DepositRequest, error) {
	stmt, err := d.db.Prepare("INSERT INTO deposit_requests (user_id, amount, memo, status, created_at) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	result, err := stmt.Exec(userID, amount, memo, StatusPending, time.Now())
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return d.GetDepositRequest(int(id))
}

// GetDepositRequest gets a deposit request by ID
func (d *Database) GetDepositRequest(id int) (*model.DepositRequest, error) {
	var req model.DepositRequest
	stmt, err := d.db.Prepare("SELECT id, user_id, amount, memo, status, created_at FROM deposit_requests WHERE id = ?")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	err = stmt.QueryRow(id).Scan(&req.ID, &req.UserID, &req.Amount, &req.Memo, &req.Status, &req.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &req, nil
}

func (d *Database) GetDepositsOfUser(userID int) ([]model.DepositRequest, error) {
	var reqs []model.DepositRequest
	stmt, err := d.db.Prepare("SELECT id, user_id, amount, memo, status, created_at FROM deposit_requests WHERE user_id = ?")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var req model.DepositRequest
		if err := rows.Scan(&req.ID, &req.UserID, &req.Amount, &req.Memo, &req.Status, &req.CreatedAt); err != nil {
			return nil, err
		}
		reqs = append(reqs, req)
	}
	return reqs, nil
}

// UpdateDepositStatus updates the status of a deposit request
func (d *Database) UpdateDepositStatus(id int, status string) error {
	stmt, err := d.db.Prepare("UPDATE deposit_requests SET status = ? WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(status, id)
	return err
}

// CreateWithdrawalRequest creates a new withdrawal request
func (d *Database) CreateWithdrawalRequest(userID int, amount float64) (sql.Result, error) {
	stmt, err := d.db.Prepare("INSERT INTO withdrawal_requests (user_id, amount, status, created_at) VALUES (?, ?, ?, ?)")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	result, err := stmt.Exec(userID, amount, StatusPending, time.Now())
	if err != nil {
		return nil, err
	}

	return result, nil
}

// ConfirmWithdrawalRequest confirms a withdrawal request
func (d *Database) ConfirmWithdrawalRequest(id int) (sql.Result, error) {
	stmt, err := d.db.Prepare("UPDATE withdrawal_requests SET status = ? WHERE id = ?")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	result, err := stmt.Exec(StatusCompleted, id)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// TODO: Func for getting withdrawal requests by user ID
func (d *Database) GetWithdrawalRequestsByUser(userID int) ([]model.WithdrawalStorage, error) {
	rows, err := d.db.Query(`
		SELECT id, user_id, amount, status, created_at, tx_hash 
		FROM withdrawals 
		WHERE user_id = ? 
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get withdrawal requests: %v", err)
	}
	defer rows.Close()

	var withdrawals []model.WithdrawalStorage
	for rows.Next() {
		var w model.WithdrawalStorage
		var txHash sql.NullString
		err := rows.Scan(&w.ID, &w.UserID, &w.Amount, &w.Status, &w.CreatedAt, &txHash)
		if err != nil {
			return nil, fmt.Errorf("failed to scan withdrawal request: %v", err)
		}
		if txHash.Valid {
			w.TxHash = txHash.String
		}
		withdrawals = append(withdrawals, w)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating withdrawal requests: %v", err)
	}

	return withdrawals, nil
}

// DB returns the underlying database connection
func (d *Database) DB() *sql.DB {
	return d.db
}

// AddOperation adds a new operation to the database
func (d *Database) AddOperation(op *model.Operation) error {
	stmt, err := d.db.Prepare(`
		INSERT INTO operations (user_id, type, amount, description, created_at, extra)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	var extraJSON []byte
	if op.Extra != nil {
		extraJSON, err = json.Marshal(op.Extra)
		if err != nil {
			return err
		}
	}

	_, err = stmt.Exec(
		op.UserID,
		op.Type,
		op.Amount,
		op.Description,
		time.Now().Unix(),
		extraJSON,
	)
	return err
}

// GetUserOperations retrieves user operations with pagination
func (d *Database) GetUserOperations(userID int, page, pageSize int) (*model.OperationHistory, error) {
	// Get total count
	var total int
	err := d.db.QueryRow("SELECT COUNT(*) FROM operations WHERE user_id = ?", userID).Scan(&total)
	if err != nil {
		return nil, err
	}

	// Calculate offset
	offset := (page - 1) * pageSize

	// Get operations
	rows, err := d.db.Query(`
		SELECT id, user_id, type, amount, description, created_at, extra
		FROM operations
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, userID, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	operations := make([]model.Operation, 0)
	for rows.Next() {
		var op model.Operation
		var extraJSON []byte
		err := rows.Scan(
			&op.ID,
			&op.UserID,
			&op.Type,
			&op.Amount,
			&op.Description,
			&op.CreatedAt,
			&extraJSON,
		)
		if err != nil {
			return nil, err
		}

		if len(extraJSON) > 0 {
			var extra interface{}
			if err := json.Unmarshal(extraJSON, &extra); err != nil {
				return nil, err
			}
			op.Extra = extra
		}

		operations = append(operations, op)
	}

	return &model.OperationHistory{
		Operations: operations,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
	}, nil
}

// UpdateWithdrawalTxHash updates the transaction hash for the latest withdrawal of a user
func (d *Database) UpdateWithdrawalTxHash(userID int, txHash string) error {
	query := `
		UPDATE withdrawals 
		SET tx_hash = ?, status = ?
		WHERE user_id = ? AND id = (
			SELECT id FROM withdrawals 
			WHERE user_id = ? 
			ORDER BY created_at DESC 
			LIMIT 1
		)`
	
	result, err := d.db.Exec(query, txHash, StatusCompleted, userID, userID)
	if err != nil {
		return fmt.Errorf("failed to update withdrawal tx hash: %v", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}

	if rows == 0 {
		return fmt.Errorf("no withdrawal found for user %d", userID)
	}

	return nil
}
