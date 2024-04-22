package store

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"
)

type Store struct {
	db *sql.DB
}

func New() (Store, error) {
	dbHost := os.Getenv("DB_HOST")
	dbUsername := os.Getenv("DB_USERNAME")
	dbPassword := os.Getenv("DB_PASSWORD")
	connectionString := "postgres://" + dbUsername + ":" + dbPassword + "@" + dbHost + ":5432" + "/protoapp_db"
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		log.Fatal(err)
		return Store{}, err
	}
	return Store{db: db}, nil
}
