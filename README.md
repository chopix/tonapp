# Build code

docker build -t tonapp-build .
docker create --name temp tonapp-build
docker cp temp:/app/tonapp ./tonapp
docker rm temp

# TonApp - Cryptocurrency Investment Platform

A robust investment platform built with Go, featuring multi-level referral system, investment management, and comprehensive operation tracking.

## Features

### Investment System
- Three investment tiers with different weekly interest rates:
  - Low: 1.5% weekly
  - Medium: 2.25% weekly
  - High: 3% weekly
- 30-day lock period for all investment types
- 20% platform fee on investment profits
- Real-time investment tracking and management

### TON Integration
- Automatic deposit address generation
- Secure withdrawal processing with transaction tracking
- Transaction hash storage and retrieval
- Support for both mainnet and testnet
- Configurable wallet versions (V4R2 supported)

### Referral System
- Three-level deep referral structure:
  - Level 1 (Direct referrals): 7% of earnings
  - Level 2 (Referrals of referrals): 3% of earnings
  - Level 3 (Third-level referrals): 1% of earnings
- Comprehensive referral statistics
- Automatic earning distribution

### Operation History
- Detailed tracking of all user operations:
  - Investment creation and closure
  - Deposits and withdrawals (including transaction hashes)
  - Referral earnings
- Rich metadata for each operation
- Pagination support for operation history
- Timestamps and detailed descriptions

### Security Features
- Admin API key authentication
- Transaction-based balance updates
- Secure withdrawal processing
- Rate limiting
- TON transaction verification

## API Endpoints

### User Management
- `POST /api/v1/users` - Create new user
- `GET /api/v1/users/by-pubkey/:pub_key` - Get user details
- `DELETE /api/v1/users/:id` - Delete user (admin only)
- `PUT /api/v1/users/:id/balance` - Update user balance (admin only)

### Investment Operations
- `POST /api/v1/users/by-pubkey/:pub_key/investments` - Create investment
- `DELETE /api/v1/users/by-pubkey/:pub_key/investments/:investment_id` - Close investment

### Referral System
- `GET /api/v1/users/by-pubkey/:pub_key/referrals` - Get referral statistics

### Operation History
- `GET /api/v1/users/by-pubkey/:pub_key/operations` - Get operation history
  - Query parameters:
    - `page` (default: 1)
    - `page_size` (default: 10, max: 100)

### Financial Operations
- `POST /api/v1/users/by-pubkey/:pub_key/deposit` - Create deposit request
- `POST /api/v1/users/by-pubkey/:pub_key/deposit/confirm` - Confirm deposit
- `POST /api/v1/users/withdraw` - Process withdrawal and return transaction hash

## API Examples

### User Management

#### Create Main User
```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{
    "pub_key": "EQBvW8Z5huBkMJYdnfAEM5JqTNkuWX3diqYENkWsIL0XggGG",
    "id": 908215144769, //optional
    "photo": "photo_url", //optional
    "name": "John Doe" //optional
  }'

Response:
{
    "success": true,
    "data": {
        "id": 908215144769,
        "pub_key": "EQBvW8Z5huBkMJYdnfAEM5JqTNkuWX3diqYENkWsIL0XggGG",
        "balance": 0,
        "ref_id": null,
        "created_at": 0
    }
}
```

#### Create Referral User
```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{
    "pub_key": "EQBKgXCNLPz0TN4lj3YKcwJHPJyCAXS4tGbgqXTUPe9aBY9G",
    "ref_id": 908215144769, 
    "id": 182275483416, //optional
    "photo": "photo_url", //optional
    "name": "John Doe" //optional
  }'

Response:
{
    "success": true,
    "data": {
        "id": 182275483416,
        "pub_key": "EQBKgXCNLPz0TN4lj3YKcwJHPJyCAXS4tGbgqXTUPe9aBY9G",
        "balance": 0,
        "ref_id": 908215144769,
        "created_at": 0
    }
}
```

#### Update User Balance (Admin Only)
```bash
curl -X PUT "http://localhost:8080/api/v1/users/182275483416/balance" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: 7d6c4d6d-7d6c-4d6d-7d6c-7d6c4d6d7d6c" \
  -d '{
    "user_id": 182275483416,
    "balance": 1500
  }'

Response:
{
    "success": true,
    "data": {
        "balance": 1500,
        "user_id": 182275483416
    }
}
```

### Investment Operations

#### Create Investment
```bash
curl -X POST "http://localhost:8080/api/v1/users/by-pubkey/EQBKgXCNLPz0TN4lj3YKcwJHPJyCAXS4tGbgqXTUPe9aBY9G/investments" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "high",
    "amount": 1000
  }'

Response:
{
    "success": true,
    "data": {
        "amount": 1000,
        "example_weekly_profit": 30,
        "lock_period": "locked for 30 days",
        "message": "investment created successfully",
        "remaining_balance": 500,
        "type": "high",
        "weekly_percent": 3
    }
}
```

### Referral System

#### Get Referral Statistics
```bash
curl http://localhost:8080/api/v1/users/by-pubkey/EQBvW8Z5huBkMJYdnfAEM5JqTNkuWX3diqYENkWsIL0XggGG/referrals

Response:
{
    "success": true,
    "data": {
        "total_referrals": 1,
        "total_earnings": 0,
        "total_earnings_usd": 0,
        "referrals_by_level": [
            {
                "user_id": 182275483416,
                "level": 1,
                "total_invested": 1000,
                "earnings_from_user": 0,
                "earnings_from_user_usd": 0,
                "level1_earnings": 0,
                "level1_earnings_usd": 0,
                "level2_earnings": 0,
                "level2_earnings_usd": 0,
                "level3_earnings": 0,
                "level3_earnings_usd": 0,
                "created_at": 0,
                "active_days": 0
            }
        ]
    }
}
```

## Configuration

Configuration is managed through `config.json`:

```json
{
    "investment_types": {
        "low": {
            "weekly_percent": 1.5,
            "min_amount": 50,
            "lock_period_days": 30
        },
        "medium": {
            "weekly_percent": 2.25,
            "min_amount": 100,
            "lock_period_days": 30
        },
        "high": {
            "weekly_percent": 3,
            "min_amount": 100,
            "lock_period_days": 30
        }
    },
    "referral_config": {
        "level1_percent": 7,
        "level2_percent": 3,
        "level3_percent": 1
    },
    "admin_api_key": "your-admin-key",
    "ton": {
        "network": "mainnet",
        "mnemonic": "your wallet mnemonic",
        "api_key": "your toncenter api key",
        "wallet_version": "V4R2",
        "fee_wallet_address": "your fee wallet address"
    },
    "rate_limit": {
        "requests_per_second": 2,
        "burst_size": 10
    }
}
```

## Configuration Example

```json
{
    "investment_types": {
        "low": {
            "weekly_percent": 1.5,
            "min_amount": 50,
            "lock_period_days": 30
        },
        "medium": {
            "weekly_percent": 2.25,
            "min_amount": 100,
            "lock_period_days": 30
        },
        "high": {
            "weekly_percent": 3,
            "min_amount": 100,
            "lock_period_days": 30
        }
    },
    "referral_config": {
        "level1_percent": 7,
        "level2_percent": 3,
        "level3_percent": 1
    },
    "admin_api_key": "your-admin-key",
    "ton": {
        "network": "mainnet",        // or "testnet"
        "mnemonic": "",             // Your wallet's mnemonic phrase
        "api_key": "",              // Your TonCenter API key
        "wallet_version": "V4R2",   // TON wallet version
        "fee_wallet_address": ""    // Address for collecting platform fees
    },
    "rate_limit": {
        "requests_per_second": 2,
        "burst_size": 10
    }
}
```

## Error Handling

All API endpoints follow a consistent error response format:

```json
{
    "success": false,
    "error": "Error message describing what went wrong"
}
```

Common HTTP status codes:
- 200: Success
- 400: Bad Request (invalid input)
- 401: Unauthorized (invalid or missing API key)
- 404: Not Found
- 429: Too Many Requests (rate limit exceeded)
- 500: Internal Server Error

## Database Schema

The application uses SQLite with the following main tables:

### Users Table
- `id` - User ID
- `pub_key` - Public key
- `balance` - Current balance
- `ref_id` - Referrer ID (optional)
- `name` - User name (optional)
- `photo` - User photo URL (optional)
- `created_at` - Creation timestamp
- `total_earnings` - Total earnings
- `current_investments` - Current investments
- `available_for_withdrawal` - Available for withdrawal

### Investments Table
- `id` - Investment ID
- `user_id` - User ID
- `type` - Investment type (low/medium/high)
- `amount` - Investment amount
- `profit` - Current profit
- `created_at` - Creation timestamp
- `status` - Investment status

### Withdrawals Table
- `id` - Withdrawal ID
- `user_id` - User ID
- `amount` - Withdrawal amount
- `status` - Status (pending/completed/failed)
- `tx_hash` - TON transaction hash
- `created_at` - Creation timestamp

### Operations Table
- `id` - Operation ID
- `user_id` - User ID
- `type` - Operation type
- `amount` - Operation amount
- `status` - Operation status
- `created_at` - Creation timestamp
- `extra` - Additional metadata (JSON)

## Getting Started

1. Clone the repository
2. Configure `config.json` with your settings:
   - Set your admin API key
   - Configure TON wallet settings:
     - Set `network` to "mainnet" or "testnet"
     - Provide your wallet `mnemonic`
     - Add your TonCenter `api_key`
     - Set `wallet_version` (V4R2 recommended)
     - Configure `fee_wallet_address` for platform fees
3. Run the application:
   ```bash
   go run cmd/main.go
   ```

## Security Notes

1. Keep your wallet mnemonic secure and never share it
2. Store your API keys securely
3. Use HTTPS in production
4. Regularly backup your database
5. Monitor withdrawal operations
6. Keep your admin API key private

## Financial Information

### User Financial Overview

The API provides three key financial metrics for each user:

1. **Total Earnings** (`total_earnings`): 
   - The total amount earned by the user from all sources (investments and referrals)
   - This value accumulates over time and represents the user's lifetime earnings

2. **Current Investments** (`current_investments`): 
   - The total amount currently invested across all active investment plans
   - This value changes as investments are created or closed

3. **Available for Withdrawal** (`available_for_withdrawal`): 
   - The maximum amount a user can withdraw at the current time
   - Calculated as 80% of total deposits minus already withdrawn amounts
   - Cannot exceed the user's current balance

These fields are automatically calculated and included in user responses when retrieving user details. These fields will always be present in the response, even if their values are zero.

### Get User Details
```bash
curl -X GET http://localhost:8080/api/v1/users/by-pubkey/EQBvW8Z5huBkMJYdnfAEM5JqTNkuWX3diqYENkWsIL0XggGG

Response:
{
    "success": true,
    "data": {
        "id": 908215144769,
        "pub_key": "EQBvW8Z5huBkMJYdnfAEM5JqTNkuWX3diqYENkWsIL0XggGG",
        "name": "John Doe",
        "photo": "photo_url",
        "balance": 0,
        "ref_id": null,
        "created_at": 1712834735,
        "total_earnings": 150.5,
        "current_investments": 1000,
        "available_for_withdrawal": 400,
        "investments": []
    }
}
```

### Example Response with Zero Values
```bash
curl -X GET http://localhost:8080/api/v1/users/by-pubkey/EQBvW8Z5huBkMJYdnfAEM5JqTNkuWX3diqYENkWsIL0XggGG

Response:
{
    "success": true,
    "data": {
        "id": 908215144769,
        "pub_key": "EQBvW8Z5huBkMJYdnfAEM5JqTNkuWX3diqYENkWsIL0XggGG",
        "name": "John Doe",
        "photo": "photo_url",
        "balance": 0,
        "ref_id": null,
        "created_at": 1712834735,
        "total_earnings": 0,
        "current_investments": 0,
        "available_for_withdrawal": 0,
        "investments": []
    }
}
```
