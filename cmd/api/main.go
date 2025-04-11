package main

import (
	"log"
	"net/http"
	"time"

	"tonapp/internal/config"
	"tonapp/internal/database"
	"tonapp/internal/handler"
	"tonapp/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Load configuration
	cfg := config.Load()

	// Initialize database
	db, err := database.New(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize handler
	h, err := handler.NewHandler(db, "config.json")
	if err != nil {
		log.Fatalf("Failed to initialize handler: %v", err)
	}

	// Initialize router
	router := setupRouter(h)

	// Create rate limiter
	rateLimiter := middleware.NewIPRateLimiter(h.GetConfig().RateLimit)

	// Apply rate limiter to all routes
	router.Use(rateLimiter.RateLimit())

	// Configure server
	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server
	log.Printf("Server starting on port %s\n", cfg.Server.Port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start server: %v\n", err)
	}
}

func setupRouter(h *handler.Handler) *gin.Engine {
	// Create default gin router
	router := gin.Default()

	// Add basic middleware
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	//Access-Control-Allow-Origin
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		// if preflight request, immediately return 200
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(200)
			return
		}

		c.Next()
	})

	// Health check endpoint
	router.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Public routes
		v1.GET("/config", func(c *gin.Context) {
			c.JSON(http.StatusOK, h.GetConfigPublic())
		})
		// User routes
		users := v1.Group("/users")
		{
			// Public routes
			users.POST("", h.CreateUser)                                     // Create new user
			users.GET("/by-pubkey/:pub_key", h.GetUser)                      // Get user by public key
			users.GET("/by-pubkey/:pub_key/referrals", h.GetReferralStats)   // Get referral stats
			users.GET("/by-pubkey/:pub_key/operations", h.GetUserOperations) // Get operation history
			users.POST("/withdraw", h.WithdrawFunds)                         // Withdraw TON to user's wallet

			// Investment routes
			users.POST("/by-pubkey/:pub_key/investments", h.CreateInvestment)
			users.DELETE("/by-pubkey/:pub_key/investments/:investment_id", h.DeleteInvestment)

			// Deposit routes
			users.POST("/by-pubkey/:pub_key/deposit", h.CreateDeposit)
			users.POST("/by-pubkey/:pub_key/deposit/confirm", h.ConfirmDeposit)

			// Admin routes
			users.DELETE("/:id", h.AdminAuth(), h.DeleteUser)             // Delete user (admin only)
			users.PUT("/:id/balance", h.AdminAuth(), h.UpdateUserBalance) // Update user balance (admin only)
		}
	}

	return router
}
