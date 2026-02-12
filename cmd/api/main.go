package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"flowpay/internal/queue"
	"flowpay/internal/server"
)

func gracefulShutdown(apiServer *http.Server, done chan bool) {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()

	log.Println("shutting down gracefully, press Ctrl+C again to force")
	stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := apiServer.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown with error: %v", err)
	}

	log.Println("Server exiting")

	done <- true
}

func main() {

	kafkaClient, err := queue.NewKafkaClient("localhost:9092")
	if err != nil {
		panic(fmt.Sprintf("failed to create Kafka client: %s", err))
	}
	defer kafkaClient.Close()

	// Logs payment events as they arrive
	err = kafkaClient.Subscribe([]string{"payments"}, func(key, value []byte) {
		log.Printf("[kafka] event received: key=%s payload=%s", string(key), string(value))
	})
	if err != nil {
		panic(fmt.Sprintf("failed to subscribe: %s", err))
	}

	srv := server.NewServer(kafkaClient)

	done := make(chan bool, 1)

	go gracefulShutdown(srv, done)

	err = srv.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		panic(fmt.Sprintf("http server error: %s", err))
	}

	<-done
	log.Println("Graceful shutdown complete.")
}
