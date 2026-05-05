package db

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

// DB wraps *sql.DB for now (keeps options open for later).
type DB struct {
	SQL *sql.DB
}

// Open performs its package-specific operation.
func Open(sqlitePath string) (*DB, error) {
	// modernc sqlite DSN: "file:<path>?_pragma=..."
	// Keep it simple and apply pragmas explicitly after open.
	dsn := fmt.Sprintf("file:%s", sqlitePath)

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Basic health check
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	// Pragmas (sane defaults for a small web backend)
	// WAL helps concurrency; foreign_keys for referential integrity; busy_timeout avoids "database is locked".
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA foreign_keys = ON;",
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA synchronous = NORMAL;",
	}

	for _, p := range pragmas {
		if _, err := sqlDB.Exec(p); err != nil {
			_ = sqlDB.Close()
			return nil, fmt.Errorf("apply pragma (%s): %w", p, err)
		}
	}

	return &DB{SQL: sqlDB}, nil
}

// Close performs its package-specific operation.
func (d *DB) Close() error {
	if d == nil || d.SQL == nil {
		return nil
	}
	return d.SQL.Close()
}

// Migrate creates tables if they do not exist.
func (d *DB) Migrate() error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}

	stmts := []string{
		`
CREATE TABLE IF NOT EXISTS users (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  password      TEXT NOT NULL DEFAULT '',
  firstname			TEXT NOT NULL DEFAULT '',
  lastname      TEXT NOT NULL DEFAULT '',
  email         TEXT NOT NULL DEFAULT '',
  locale        TEXT NOT NULL DEFAULT 'en-US',
  status        TEXT NOT NULL DEFAULT 'active',
  settings      TEXT NOT NULL DEFAULT '{}',
  created_at    INTEGER NOT NULL DEFAULT 0,
  updated_at    INTEGER NOT NULL DEFAULT 0
);
`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email);`,
		`CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);`,

		`
CREATE TABLE IF NOT EXISTS groups (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  name        TEXT NOT NULL DEFAULT '',
  created_at  INTEGER NOT NULL DEFAULT 0,
  updated_at  INTEGER NOT NULL DEFAULT 0
);
`,
		// group_id = 1 is reserved as the system group (Unix convention, analogous to gid=0).
		`INSERT OR IGNORE INTO groups (id, name, created_at, updated_at) VALUES (1, 'System', 0, 0);`,

		`
CREATE TABLE IF NOT EXISTS group_users (
  group_id   INTEGER NOT NULL,
  user_id    INTEGER NOT NULL,
  PRIMARY KEY (group_id, user_id),
  FOREIGN KEY(group_id) REFERENCES groups(id),
  FOREIGN KEY(user_id)  REFERENCES users(id)
);
`,
		`CREATE INDEX IF NOT EXISTS idx_group_users_user_id ON group_users(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_group_users_group_id ON group_users(group_id);`,

		`
CREATE TABLE IF NOT EXISTS memberships (
  entity_type  TEXT    NOT NULL,
  entity_id    INTEGER NOT NULL,
  user_id      INTEGER NOT NULL,
  role         TEXT    NOT NULL DEFAULT '',
  created_at   INTEGER NOT NULL DEFAULT 0,
  updated_at   INTEGER NOT NULL DEFAULT 0,

  PRIMARY KEY (entity_type, entity_id, user_id),
  FOREIGN KEY(user_id) REFERENCES users(id)
);
`,
		`CREATE INDEX IF NOT EXISTS idx_memberships_user_id ON memberships(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_memberships_entity ON memberships(entity_type, entity_id);`,

		`
CREATE TABLE IF NOT EXISTS depots (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  name           TEXT NOT NULL DEFAULT '',
  broker_name    TEXT NOT NULL DEFAULT '',
  account_number TEXT NOT NULL DEFAULT '',
  base_currency  TEXT NOT NULL DEFAULT 'EUR',
  description    TEXT NOT NULL DEFAULT '',
  status         TEXT NOT NULL DEFAULT 'active',
  created_at     INTEGER NOT NULL DEFAULT 0,
  updated_at     INTEGER NOT NULL DEFAULT 0
);
`,
		`CREATE INDEX IF NOT EXISTS idx_depots_status ON depots(status);`,

		`
CREATE TABLE IF NOT EXISTS securities (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  group_id      INTEGER NOT NULL,
  name          TEXT NOT NULL DEFAULT '',
  isin          TEXT NOT NULL DEFAULT '',
  wkn           TEXT NOT NULL DEFAULT '',
  symbol        TEXT NOT NULL DEFAULT '',
  status        TEXT NOT NULL DEFAULT 'active',
  created_at    INTEGER NOT NULL DEFAULT 0,
  updated_at    INTEGER NOT NULL DEFAULT 0
);
`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_securities_group_isin ON securities(group_id, isin);`,
		`CREATE INDEX IF NOT EXISTS idx_securities_group_wkn ON securities(group_id, wkn);`,
		`CREATE INDEX IF NOT EXISTS idx_securities_group_symbol ON securities(group_id, symbol);`,
		`CREATE INDEX IF NOT EXISTS idx_securities_group_status ON securities(group_id, status);`,

		`
CREATE TABLE IF NOT EXISTS withholding_tax_defaults (
  id                                      INTEGER PRIMARY KEY AUTOINCREMENT,
  group_id                                INTEGER NOT NULL,
  depot_id                                INTEGER NOT NULL DEFAULT 0,
  country_code                            TEXT NOT NULL DEFAULT '',
  country_name                            TEXT NOT NULL DEFAULT '',
  withholding_tax_percent_default         TEXT NOT NULL DEFAULT '',
  withholding_tax_percent_credit_default  TEXT NOT NULL DEFAULT '',
  created_at                              INTEGER NOT NULL DEFAULT 0,
  updated_at                              INTEGER NOT NULL DEFAULT 0
);
`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_withholding_tax_defaults_group_depot_country ON withholding_tax_defaults(group_id, depot_id, country_code);`,
		`CREATE INDEX IF NOT EXISTS idx_withholding_tax_defaults_group_id ON withholding_tax_defaults(group_id);`,
		`CREATE INDEX IF NOT EXISTS idx_withholding_tax_defaults_depot_id ON withholding_tax_defaults(depot_id);`,
		`CREATE INDEX IF NOT EXISTS idx_withholding_tax_defaults_group_country ON withholding_tax_defaults(group_id, country_code);`,

		`
CREATE TABLE IF NOT EXISTS dividend_entries (
  id                                       INTEGER PRIMARY KEY AUTOINCREMENT,

  depot_id                                 INTEGER NOT NULL,
  security_id                              INTEGER NOT NULL,

  pay_date                                 TEXT NOT NULL DEFAULT '',
  ex_date                                  TEXT NOT NULL DEFAULT '',

  security_name                            TEXT NOT NULL DEFAULT '',
  security_isin                            TEXT NOT NULL DEFAULT '',
  security_wkn                             TEXT NOT NULL DEFAULT '',
  security_symbol                          TEXT NOT NULL DEFAULT '',

  quantity                                 TEXT NOT NULL DEFAULT '',

  dividend_per_unit_amount                 TEXT NOT NULL DEFAULT '',
  dividend_per_unit_currency               TEXT NOT NULL DEFAULT '',

  fx_rate_label                            TEXT NOT NULL DEFAULT '',
  fx_rate                                  TEXT NOT NULL DEFAULT '1',

  gross_amount                             TEXT NOT NULL DEFAULT '',
  gross_currency                           TEXT NOT NULL DEFAULT '',

  payout_amount                            TEXT NOT NULL DEFAULT '',
  payout_currency                          TEXT NOT NULL DEFAULT '',

  withholding_tax_country_code             TEXT NOT NULL DEFAULT '',
  withholding_tax_percent                  TEXT NOT NULL DEFAULT '',

  withholding_tax_amount                   TEXT NOT NULL DEFAULT '',
  withholding_tax_currency                 TEXT NOT NULL DEFAULT '',

  withholding_tax_amount_credit            TEXT NOT NULL DEFAULT '',
  withholding_tax_amount_credit_currency   TEXT NOT NULL DEFAULT '',

  withholding_tax_amount_refundable        TEXT NOT NULL DEFAULT '',
  withholding_tax_amount_refundable_currency TEXT NOT NULL DEFAULT '',

  inland_tax_amount                        TEXT NOT NULL DEFAULT '',
  inland_tax_currency                      TEXT NOT NULL DEFAULT '',
  inland_tax_details                       TEXT NOT NULL DEFAULT '',

  foreign_fees_amount                      TEXT NOT NULL DEFAULT '',
  foreign_fees_currency                    TEXT NOT NULL DEFAULT '',

  note                                     TEXT NOT NULL DEFAULT '',

  calc_gross_amount_base                   TEXT NOT NULL DEFAULT '',
  calc_after_withholding_amount_base       TEXT NOT NULL DEFAULT '',

  created_at                               INTEGER NOT NULL DEFAULT 0,
  updated_at                               INTEGER NOT NULL DEFAULT 0,

  FOREIGN KEY(depot_id) REFERENCES depots(id),
  FOREIGN KEY(security_id) REFERENCES securities(id)
);
`,
		`CREATE INDEX IF NOT EXISTS idx_dividend_entries_depot_id ON dividend_entries(depot_id);`,
		`CREATE INDEX IF NOT EXISTS idx_dividend_entries_security_id ON dividend_entries(security_id);`,
		`CREATE INDEX IF NOT EXISTS idx_dividend_entries_pay_date ON dividend_entries(pay_date);`,
		`CREATE INDEX IF NOT EXISTS idx_dividend_entries_ex_date ON dividend_entries(ex_date);`,
		`CREATE INDEX IF NOT EXISTS idx_dividend_entries_depot_pay_date ON dividend_entries(depot_id, pay_date);`,
		`CREATE INDEX IF NOT EXISTS idx_dividend_entries_security_pay_date ON dividend_entries(security_id, pay_date);`,
		`CREATE INDEX IF NOT EXISTS idx_dividend_entries_security_isin ON dividend_entries(security_isin);`,
		`CREATE INDEX IF NOT EXISTS idx_dividend_entries_withholding_country_code ON dividend_entries(withholding_tax_country_code);`,

		`
CREATE TABLE IF NOT EXISTS currencies (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  group_id       INTEGER NOT NULL DEFAULT 0,
  currency       TEXT NOT NULL DEFAULT '',
  name           TEXT NOT NULL DEFAULT '',
  decimal_places INTEGER NOT NULL DEFAULT 2,
  status         TEXT NOT NULL DEFAULT 'active',
  created_at     INTEGER NOT NULL DEFAULT 0,
  updated_at     INTEGER NOT NULL DEFAULT 0
);
`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_currencies_group_currency ON currencies(group_id, currency);`,
		`CREATE INDEX IF NOT EXISTS idx_currencies_group_status ON currencies(group_id, status);`,
		`
INSERT OR IGNORE INTO currencies (group_id, currency, name, decimal_places, created_at, updated_at)
VALUES (0, 'EUR', 'Euro', 2, 0, 0);
`,
		`
INSERT OR IGNORE INTO currencies (group_id, currency, name, decimal_places, created_at, updated_at)
VALUES (0, 'USD', 'US Dollar', 2, 0, 0);
`,

		`
CREATE TABLE IF NOT EXISTS audit_log (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id      INTEGER NOT NULL DEFAULT 0,
  action       TEXT    NOT NULL DEFAULT '',
  entity_type  TEXT    NOT NULL DEFAULT '',
  entity_id    INTEGER NOT NULL DEFAULT 0,
  detail       TEXT    NOT NULL DEFAULT '',
  created_at   INTEGER NOT NULL DEFAULT 0
);
`,
		`CREATE INDEX IF NOT EXISTS idx_audit_log_user_id ON audit_log(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_audit_log_entity ON audit_log(entity_type, entity_id);`,
		`CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON audit_log(created_at);`,
	}

	for _, s := range stmts {
		if _, err := d.SQL.Exec(s); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}

	log.Println("sqlite migration done")
	return nil
}
