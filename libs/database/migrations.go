package database

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// Migration represents a database migration
type Migration struct {
	Version     int
	Description string
	Up          func(tx *sql.Tx) error
	Down        func(tx *sql.Tx) error
}

// Migrator handles database migrations
type Migrator struct {
	db         *DB
	migrations []Migration
}

// NewMigrator creates a new migrator
func NewMigrator(db *DB) *Migrator {
	return &Migrator{
		db:         db,
		migrations: make([]Migration, 0),
	}
}

// AddMigration adds a migration to the migrator
func (m *Migrator) AddMigration(migration Migration) {
	m.migrations = append(m.migrations, migration)
}

// Migrate runs all pending migrations
func (m *Migrator) Migrate(ctx context.Context) error {
	// Ensure migrations table exists
	if err := m.createMigrationsTable(ctx); err != nil {
		return err
	}

	// Get current version
	currentVersion, err := m.getCurrentVersion(ctx)
	if err != nil {
		return err
	}

	// Sort migrations by version
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version < m.migrations[j].Version
	})

	// Run pending migrations
	for _, migration := range m.migrations {
		if migration.Version <= currentVersion {
			continue
		}

		if err := m.runMigration(ctx, migration, true); err != nil {
			return fmt.Errorf("migration %d failed: %w", migration.Version, err)
		}
	}

	return nil
}

// Rollback rolls back the last migration
func (m *Migrator) Rollback(ctx context.Context) error {
	currentVersion, err := m.getCurrentVersion(ctx)
	if err != nil {
		return err
	}

	if currentVersion == 0 {
		return nil // Nothing to rollback
	}

	// Find the migration to rollback
	for _, migration := range m.migrations {
		if migration.Version == currentVersion {
			return m.runMigration(ctx, migration, false)
		}
	}

	return fmt.Errorf("migration version %d not found", currentVersion)
}

// createMigrationsTable creates the migrations tracking table
func (m *Migrator) createMigrationsTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			description TEXT NOT NULL,
			applied_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`
	_, err := m.db.ExecContext(ctx, query)
	return err
}

// getCurrentVersion gets the current migration version
func (m *Migrator) getCurrentVersion(ctx context.Context) (int, error) {
	var version int
	query := `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`
	err := m.db.QueryRowContext(ctx, query).Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}

// runMigration runs a single migration
func (m *Migrator) runMigration(ctx context.Context, migration Migration, up bool) error {
	return m.db.Transaction(ctx, func(tx *sql.Tx) error {
		if up {
			if err := migration.Up(tx); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx,
				`INSERT INTO schema_migrations (version, description, applied_at) VALUES ($1, $2, $3)`,
				migration.Version, migration.Description, time.Now().UTC())
			return err
		} else {
			if err := migration.Down(tx); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx,
				`DELETE FROM schema_migrations WHERE version = $1`,
				migration.Version)
			return err
		}
	})
}

// GetAppliedMigrations returns all applied migrations
func (m *Migrator) GetAppliedMigrations(ctx context.Context) ([]int, error) {
	query := `SELECT version FROM schema_migrations ORDER BY version`
	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}

	return versions, rows.Err()
}

// DefaultMigrations returns the default schema migrations
func DefaultMigrations() []Migration {
	return []Migration{
		{
			Version:     1,
			Description: "Create accounts table",
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec(`
					CREATE TABLE IF NOT EXISTS accounts (
						id VARCHAR(64) PRIMARY KEY,
						email VARCHAR(255) UNIQUE NOT NULL,
						type VARCHAR(32) NOT NULL,
						status VARCHAR(32) NOT NULL DEFAULT 'pending',
						kyc_status VARCHAR(32) NOT NULL DEFAULT 'not_started',
						profile JSONB,
						balance BIGINT NOT NULL DEFAULT 0,
						currency VARCHAR(3) NOT NULL DEFAULT 'USD',
						metadata JSONB,
						created_at TIMESTAMP NOT NULL DEFAULT NOW(),
						updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
						verified_at TIMESTAMP
					);
					CREATE INDEX idx_accounts_email ON accounts(email);
					CREATE INDEX idx_accounts_status ON accounts(status);
				`)
				return err
			},
			Down: func(tx *sql.Tx) error {
				_, err := tx.Exec(`DROP TABLE IF EXISTS accounts`)
				return err
			},
		},
		{
			Version:     2,
			Description: "Create transactions table",
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec(`
					CREATE TABLE IF NOT EXISTS transactions (
						id VARCHAR(64) PRIMARY KEY,
						idempotency_key VARCHAR(255) UNIQUE,
						amount BIGINT NOT NULL,
						currency VARCHAR(3) NOT NULL,
						status VARCHAR(32) NOT NULL DEFAULT 'pending',
						payment_method VARCHAR(32) NOT NULL,
						merchant_id VARCHAR(64) NOT NULL,
						customer_id VARCHAR(64) NOT NULL,
						description TEXT,
						metadata JSONB,
						provider_ref VARCHAR(255),
						failure_reason TEXT,
						refunded_amount BIGINT NOT NULL DEFAULT 0,
						created_at TIMESTAMP NOT NULL DEFAULT NOW(),
						updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
						completed_at TIMESTAMP
					);
					CREATE INDEX idx_transactions_idempotency ON transactions(idempotency_key);
					CREATE INDEX idx_transactions_merchant ON transactions(merchant_id);
					CREATE INDEX idx_transactions_status ON transactions(status);
					CREATE INDEX idx_transactions_created ON transactions(created_at);
				`)
				return err
			},
			Down: func(tx *sql.Tx) error {
				_, err := tx.Exec(`DROP TABLE IF EXISTS transactions`)
				return err
			},
		},
		{
			Version:     3,
			Description: "Create ledger tables",
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec(`
					CREATE TABLE IF NOT EXISTS ledger_accounts (
						id VARCHAR(64) PRIMARY KEY,
						name VARCHAR(255) NOT NULL,
						type VARCHAR(32) NOT NULL,
						currency VARCHAR(3) NOT NULL DEFAULT 'USD',
						balance BIGINT NOT NULL DEFAULT 0,
						normal_balance VARCHAR(8) NOT NULL,
						is_active BOOLEAN NOT NULL DEFAULT true,
						created_at TIMESTAMP NOT NULL DEFAULT NOW(),
						updated_at TIMESTAMP NOT NULL DEFAULT NOW()
					);

					CREATE TABLE IF NOT EXISTS journal_entries (
						id VARCHAR(64) PRIMARY KEY,
						description TEXT NOT NULL,
						reference_type VARCHAR(64),
						reference_id VARCHAR(64),
						metadata JSONB,
						created_at TIMESTAMP NOT NULL DEFAULT NOW()
					);

					CREATE TABLE IF NOT EXISTS ledger_entries (
						id VARCHAR(64) PRIMARY KEY,
						journal_id VARCHAR(64) NOT NULL REFERENCES journal_entries(id),
						account_id VARCHAR(64) NOT NULL REFERENCES ledger_accounts(id),
						type VARCHAR(8) NOT NULL,
						amount BIGINT NOT NULL,
						currency VARCHAR(3) NOT NULL,
						balance BIGINT NOT NULL,
						description TEXT,
						reference_type VARCHAR(64),
						reference_id VARCHAR(64),
						metadata JSONB,
						created_at TIMESTAMP NOT NULL DEFAULT NOW()
					);
					CREATE INDEX idx_ledger_entries_journal ON ledger_entries(journal_id);
					CREATE INDEX idx_ledger_entries_account ON ledger_entries(account_id);
					CREATE INDEX idx_ledger_entries_reference ON ledger_entries(reference_type, reference_id);
				`)
				return err
			},
			Down: func(tx *sql.Tx) error {
				_, err := tx.Exec(`
					DROP TABLE IF EXISTS ledger_entries;
					DROP TABLE IF EXISTS journal_entries;
					DROP TABLE IF EXISTS ledger_accounts;
				`)
				return err
			},
		},
		{
			Version:     4,
			Description: "Create notifications table",
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec(`
					CREATE TABLE IF NOT EXISTS notifications (
						id VARCHAR(64) PRIMARY KEY,
						type VARCHAR(32) NOT NULL,
						status VARCHAR(32) NOT NULL DEFAULT 'queued',
						event VARCHAR(64) NOT NULL,
						recipient VARCHAR(512) NOT NULL,
						subject VARCHAR(512),
						content TEXT NOT NULL,
						metadata JSONB,
						retry_count INTEGER NOT NULL DEFAULT 0,
						max_retries INTEGER NOT NULL DEFAULT 3,
						last_error TEXT,
						scheduled_at TIMESTAMP,
						sent_at TIMESTAMP,
						created_at TIMESTAMP NOT NULL DEFAULT NOW(),
						updated_at TIMESTAMP NOT NULL DEFAULT NOW()
					);
					CREATE INDEX idx_notifications_status ON notifications(status);
					CREATE INDEX idx_notifications_event ON notifications(event);
					CREATE INDEX idx_notifications_scheduled ON notifications(scheduled_at);
				`)
				return err
			},
			Down: func(tx *sql.Tx) error {
				_, err := tx.Exec(`DROP TABLE IF EXISTS notifications`)
				return err
			},
		},
		{
			Version:     5,
			Description: "Create audit_logs table",
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec(`
					CREATE TABLE IF NOT EXISTS audit_logs (
						id VARCHAR(64) PRIMARY KEY,
						action VARCHAR(64) NOT NULL,
						entity_type VARCHAR(64) NOT NULL,
						entity_id VARCHAR(64) NOT NULL,
						actor_type VARCHAR(32) NOT NULL,
						actor_id VARCHAR(64) NOT NULL,
						old_value JSONB,
						new_value JSONB,
						ip_address INET,
						user_agent TEXT,
						metadata JSONB,
						created_at TIMESTAMP NOT NULL DEFAULT NOW()
					);
					CREATE INDEX idx_audit_logs_entity ON audit_logs(entity_type, entity_id);
					CREATE INDEX idx_audit_logs_actor ON audit_logs(actor_type, actor_id);
					CREATE INDEX idx_audit_logs_created ON audit_logs(created_at);
				`)
				return err
			},
			Down: func(tx *sql.Tx) error {
				_, err := tx.Exec(`DROP TABLE IF EXISTS audit_logs`)
				return err
			},
		},
	}
}
