package store

import (
	"base-server/internal/observability"
	"errors"
	"log"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // Import the pgx stdlib for sqlx
	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	db     *sqlx.DB
	logger *observability.Logger
}

func New(connectionString string, logger *observability.Logger) (Store, error) {
	db, err := sqlx.Open("pgx", connectionString)
	if err != nil {
		log.Fatal(err)
		return Store{}, err
	}

	// Configure connection pool to prevent exhaustion
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	return Store{db: db, logger: logger}, nil
}

// GetDB returns the underlying database connection (useful for tests)
func (s *Store) GetDB() *sqlx.DB {
	return s.db
}
