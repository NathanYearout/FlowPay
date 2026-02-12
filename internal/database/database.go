package database

import (
	"context"
	"embed"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/joho/godotenv/autoload"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Service interface {
	Health() map[string]string
	Close()
	Pool() *pgxpool.Pool
}

type service struct {
	pool *pgxpool.Pool
}

var (
	database   = os.Getenv("BLUEPRINT_DB_DATABASE")
	password   = os.Getenv("BLUEPRINT_DB_PASSWORD")
	username   = os.Getenv("BLUEPRINT_DB_USERNAME")
	port       = os.Getenv("BLUEPRINT_DB_PORT")
	host       = os.Getenv("BLUEPRINT_DB_HOST")
	schema     = os.Getenv("BLUEPRINT_DB_SCHEMA")
	dbInstance *service
)

func New() Service {
	if dbInstance != nil {
		return dbInstance
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&search_path=%s", username, password, host, port, database, schema)

	runMigrations(connStr)

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		log.Fatalf("failed to create connection pool: %v", err)
	}

	dbInstance = &service{pool: pool}
	return dbInstance
}

func (s *service) Pool() *pgxpool.Pool {
	return s.pool
}

func runMigrations(connStr string) {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		log.Fatalf("failed to read migrations: %v", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, connStr)
	if err != nil {
		log.Fatalf("failed to create migrate instance: %v", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("migration failed: %v", err)
	}

	log.Println("migrations applied successfully")
}

func (s *service) Health() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	stats := make(map[string]string)

	err := s.pool.Ping(ctx)
	if err != nil {
		stats["status"] = "down"
		stats["error"] = fmt.Sprintf("db down: %v", err)
		log.Fatalf("db down: %v", err)
		return stats
	}

	stats["status"] = "up"
	stats["message"] = "It's healthy"

	poolStats := s.pool.Stat()
	stats["total_connections"] = strconv.FormatInt(int64(poolStats.TotalConns()), 10)
	stats["idle_connections"] = strconv.FormatInt(int64(poolStats.IdleConns()), 10)
	stats["acquired_connections"] = strconv.FormatInt(int64(poolStats.AcquiredConns()), 10)

	if poolStats.TotalConns() > 40 {
		stats["message"] = "The database is experiencing heavy load."
	}

	return stats
}

func (s *service) Close() {
	log.Printf("Disconnected from database: %s", database)
	s.pool.Close()
}
