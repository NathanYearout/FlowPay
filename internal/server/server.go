package server

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/joho/godotenv/autoload"

	"flowpay/internal/database"
	"flowpay/internal/queue"
)

type Server struct {
	port int
	db   database.Service
	queue *queue.KafkaClient
}

func NewServer(q *queue.KafkaClient) *http.Server {
	port, _ := strconv.Atoi(os.Getenv("PORT"))
	if port == 0 {
		port = 8080
	}

	s := &Server{
		port:  port,
		db:    database.New(),
		queue: q,
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
