package store

import (
	"base-server/internal/observability"
	"errors"
	"log"

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
	return Store{db: db, logger: logger}, nil
}

// DB returns the underlying database connection
func (s *Store) DB() *sqlx.DB {
	return s.db
}
