package store

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

type Store struct {
	db *sql.DB
}

func New() (Store, error) {
	connStr := "postgres://base-user:basepassword@localhost:5432/base-server?sslmode=verify-full"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
		return Store{}, err
	}
	return Store{db: db}, nil
}
