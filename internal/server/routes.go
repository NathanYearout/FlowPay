package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"

	"flowpay/internal/ledger"
)

func (s *Server) RegisterRoutes() http.Handler {
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type", "Idempotency-Key"},
		AllowCredentials: true,
	}))

	r.GET("/health", s.healthHandler)

	r.POST("/accounts", s.createAccountHandler)
	r.GET("/accounts/:id/balance", s.getBalanceHandler)
	r.POST("/transactions", s.createTransactionHandler)

	return r
}

func (s *Server) healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, s.db.Health())
}

type createAccountRequest struct {
	Owner       string `json:"owner" binding:"required"`
	AssetType   string `json:"asset_type" binding:"required"`
	AccountType string `json:"account_type" binding:"required"`
}

func (s *Server) createAccountHandler(c *gin.Context) {
	var req createAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	account, err := s.ledger.CreateAccount(c.Request.Context(), req.Owner, req.AssetType, req.AccountType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create account"})
		return
	}

	c.JSON(http.StatusCreated, account)
}

func (s *Server) getBalanceHandler(c *gin.Context) {
	id := c.Param("id")

	uid := pgtype.UUID{}
	if err := uid.Scan(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	balance, err := s.ledger.GetBalance(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get balance"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"account_id": id, "balance": balance})
}

type createTransactionRequest struct {
	Type    string              `json:"type" binding:"required"`
	Entries []ledger.EntryInput `json:"entries" binding:"required,min=2"`
}

func (s *Server) createTransactionHandler(c *gin.Context) {
	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Idempotency-Key header is required"})
		return
	}

	var req createTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	metadata, _ := json.Marshal(map[string]interface{}{
		"type":    req.Type,
		"entries": req.Entries,
	})

	txn, entries, err := s.ledger.CreateTransaction(
		c.Request.Context(),
		idempotencyKey,
		req.Type,
		metadata,
		req.Entries,
	)
	if err != nil {
		switch {
		case errors.Is(err, ledger.ErrEntriesUnbalanced):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ledger.ErrMixedAssets):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ledger.ErrAccountNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create transaction"})
		}
		return
	}

	// Publish event after Postgres commit, Kafka isn't source of truth
	event := map[string]interface{}{
		"event":          "payment.completed",
		"transaction_id": txn.ID,
		"type":           txn.Type,
		"status":         txn.Status,
		"entries":        entries,
	}
	if err := s.queue.Publish("payments", txn.IdempotencyKey, event); err != nil {
		log.Printf("failed to publish event: %v", err)
	}

	c.JSON(http.StatusCreated, gin.H{
		"transaction": txn,
		"entries":     entries,
	})
}
