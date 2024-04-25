package store

import (
	"base-server/internal/observability"
	"log"

	_ "github.com/jackc/pgx/v4/stdlib" // Import the pgx stdlib for sqlx
	"github.com/jmoiron/sqlx"
)

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
