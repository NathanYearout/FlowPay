package server

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/joho/godotenv/autoload"

	"flowpay/internal/database"
	"flowpay/internal/ledger"
	"flowpay/internal/queue"
)

type Server struct {
	port   int
	db     database.Service
	ledger *ledger.Ledger
	queue  *queue.KafkaClient
}

func NewServer(q *queue.KafkaClient) *http.Server {
	port, _ := strconv.Atoi(os.Getenv("PORT"))
	if port == 0 {
		port = 8080
	}

	db := database.New()

	s := &Server{
		port:   port,
		db:     db,
		ledger: ledger.New(db.Pool()),
		queue:  q,
	}

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      s.RegisterRoutes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return server
}
