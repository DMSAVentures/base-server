package store

import (
	"base-server/internal/observability"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
)

// TestDBType represents the type of database to use for testing
type TestDBType string

const (
	TestDBTypePostgres TestDBType = "postgres"
)

// TestDB wraps a test database instance
type TestDB struct {
	db     *sqlx.DB
	logger *observability.Logger
	Store  Store
	dbType TestDBType
}

// SetupTestDB creates a new test database instance
// It supports both dockerized PostgreSQL and in-memory testing
func SetupTestDB(t *testing.T, dbType TestDBType) *TestDB {
	t.Helper()

	// Use environment variable to determine which database to use
	// If not set, default to postgres
	if dbType == "" {
		envDBType := os.Getenv("TEST_DB_TYPE")
		if envDBType == "" {
			dbType = TestDBTypePostgres
		} else {
			dbType = TestDBType(envDBType)
		}
	}

	logger := observability.NewLogger()

	var db *sqlx.DB
	var err error

	switch dbType {
	case TestDBTypePostgres:
		db, err = setupPostgresDB(t)
	default:
		t.Fatalf("unsupported database type: %s", dbType)
	}

	if err != nil {
		t.Fatalf("failed to setup test database: %v", err)
	}

	// Migrations are handled by Flyway in docker-compose.services.yml
	// No need to run migrations here

	store := Store{db: db, logger: logger}

	return &TestDB{
		db:     db,
		logger: logger,
		Store:  store,
		dbType: dbType,
	}
}

// setupPostgresDB creates a PostgreSQL database connection
// It will use a dockerized database if TEST_DB_HOST is set
// Otherwise it expects a running PostgreSQL instance
func setupPostgresDB(t *testing.T) (*sqlx.DB, error) {
	t.Helper()

	// Check if we should use Docker or existing database
	dbHost := os.Getenv("TEST_DB_HOST")
	dbPort := os.Getenv("TEST_DB_PORT")
	dbUser := os.Getenv("TEST_DB_USER")
	dbPass := os.Getenv("TEST_DB_PASSWORD")
	dbName := os.Getenv("TEST_DB_NAME")

	// Use defaults if not set (matching docker-compose.services.yml)
	if dbHost == "" {
		dbHost = "localhost"
	}
	if dbPort == "" {
		dbPort = "5432"
	}
	if dbUser == "" {
		dbUser = "base_user"
	}
	if dbPass == "" {
		dbPass = "base_password"
	}
	if dbName == "" {
		dbName = "base_db"
	}

	// Connect to the existing database
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPass, dbHost, dbPort, dbName)

	db, err := sqlx.Open("pgx", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Register cleanup to close connection
	t.Cleanup(func() {
		db.Close()
	})

	return db, nil
}

// runMigrations applies all migration files to the database
func runMigrations(db *sqlx.DB) error {
	// Find migrations directory
	migrationsDir := "../../migrations"
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		// Try absolute path
		migrationsDir = "migrations"
		if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
			return fmt.Errorf("migrations directory not found")
		}
	}

	// Read all migration files
	files, err := filepath.Glob(filepath.Join(migrationsDir, "V*.sql"))
	if err != nil {
		return fmt.Errorf("failed to read migration files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no migration files found in %s", migrationsDir)
	}

	// Sort files by version
	sort.Strings(files)

	// Execute each migration
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", file, err)
		}

		// Execute migration
		_, err = db.Exec(string(content))
		if err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", filepath.Base(file), err)
		}
	}

	return nil
}

// Truncate clears all data from tables while preserving schema
func (tdb *TestDB) Truncate(t *testing.T, tables ...string) {
	t.Helper()

	if len(tables) == 0 {
		// Truncate all tables (in reverse dependency order)
		tables = []string{
			"webhook_deliveries",
			"webhooks",
			"fraud_detections",
			"audit_logs",
			"api_keys",
			"campaign_analytics",
			"user_activity_logs",
			"email_logs",
			"email_templates",
			"user_rewards",
			"rewards",
			"referrals",
			"waitlist_users",
			"campaigns",
			"team_members",
			"accounts",
			"usage_logs",
			"messages",
			"conversations",
			"payment_method",
			"subscriptions",
			"prices",
			"products",
			"plan_feature_limits",
			"limits",
			"features",
			"oauth_auth",
			"email_auth",
			"user_auth",
			"users",
		}
	}

	for _, table := range tables {
		_, err := tdb.db.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		if err != nil {
			// Skip if table doesn't exist
			if !strings.Contains(err.Error(), "does not exist") {
				t.Fatalf("failed to truncate table %s: %v", table, err)
			}
		}
	}
}

// Close closes the database connection
func (tdb *TestDB) Close() error {
	return tdb.db.Close()
}

// GetDB returns the underlying sqlx.DB for direct access if needed
func (tdb *TestDB) GetDB() *sqlx.DB {
	return tdb.db
}

// ExecSQL executes raw SQL for test setup
func (tdb *TestDB) ExecSQL(t *testing.T, query string, args ...interface{}) sql.Result {
	t.Helper()
	result, err := tdb.db.Exec(query, args...)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v", err)
	}
	return result
}

// MustExec executes SQL and fails the test if there's an error
func (tdb *TestDB) MustExec(t *testing.T, query string, args ...interface{}) {
	t.Helper()
	_, err := tdb.db.Exec(query, args...)
	if err != nil {
		t.Fatalf("failed to execute SQL: %v", err)
	}
}

// WithContext returns a context for testing
func (tdb *TestDB) WithContext() context.Context {
	return context.Background()
}
