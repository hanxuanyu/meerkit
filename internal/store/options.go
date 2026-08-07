package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/schema"
	_ "modernc.org/sqlite"
)

type DatabaseType string

const (
	DatabaseSQLite DatabaseType = "sqlite"
	DatabaseMySQL  DatabaseType = "mysql"
)

type Options struct {
	Type            DatabaseType
	DSN             string
	DataDir         string
	AutoMigrate     bool
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// OpenStore keeps tests and embedded callers concise while using the same
// connection and migration path as the configured application.
func OpenStore(dataDir string) (*Store, error) {
	return Open(context.Background(), Options{Type: DatabaseSQLite, DataDir: dataDir, AutoMigrate: true})
}

func Open(ctx context.Context, options Options) (*Store, error) {
	options, err := normalizeOptions(options)
	if err != nil {
		return nil, err
	}

	driverName, dsn, dialect, err := connectionSettings(options)
	if err != nil {
		return nil, err
	}
	sqlDB, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s database: %w", options.Type, err)
	}
	configurePool(sqlDB, options)
	orm := bun.NewDB(sqlDB, dialect)
	store := &Store{db: sqlDB, orm: orm, databaseType: options.Type}
	if err := store.Ping(ctx); err != nil {
		_ = orm.Close()
		return nil, fmt.Errorf("connect to %s database: %w", options.Type, err)
	}
	if options.AutoMigrate {
		if err := store.migrate(ctx); err != nil {
			_ = orm.Close()
			return nil, fmt.Errorf("migrate %s database: %w", options.Type, err)
		}
	} else if err := store.validateSchema(ctx); err != nil {
		_ = orm.Close()
		return nil, fmt.Errorf("validate %s database schema: %w", options.Type, err)
	}
	return store, nil
}

func normalizeOptions(options Options) (Options, error) {
	options.Type = DatabaseType(strings.ToLower(strings.TrimSpace(string(options.Type))))
	if options.Type == "" {
		options.Type = DatabaseSQLite
	}
	switch options.Type {
	case DatabaseSQLite:
		if strings.TrimSpace(options.DSN) == "" {
			if strings.TrimSpace(options.DataDir) == "" {
				return options, errors.New("sqlite requires data directory or DSN")
			}
			if err := os.MkdirAll(options.DataDir, 0o750); err != nil {
				return options, err
			}
			path := filepath.Join(options.DataDir, "meerkit.db")
			options.DSN = fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", filepath.ToSlash(path))
		}
		if options.MaxOpenConns == 0 {
			options.MaxOpenConns = 4
		}
		if options.MaxIdleConns == 0 {
			options.MaxIdleConns = options.MaxOpenConns
		}
	case DatabaseMySQL:
		if strings.TrimSpace(options.DSN) == "" {
			return options, errors.New("mysql requires DSN")
		}
		if options.MaxOpenConns == 0 {
			options.MaxOpenConns = 25
		}
		if options.MaxIdleConns == 0 {
			options.MaxIdleConns = min(10, options.MaxOpenConns)
		}
		if options.ConnMaxLifetime == 0 {
			options.ConnMaxLifetime = 30 * time.Minute
		}
		if options.ConnMaxIdleTime == 0 {
			options.ConnMaxIdleTime = 5 * time.Minute
		}
		if options.MaxOpenConns < 2 {
			return options, errors.New("mysql max_open_conns must be at least 2")
		}
	default:
		return options, fmt.Errorf("unsupported database type %q", options.Type)
	}
	if options.MaxOpenConns < 1 || options.MaxIdleConns < 0 || options.MaxIdleConns > options.MaxOpenConns {
		return options, errors.New("invalid database connection pool limits")
	}
	return options, nil
}

func connectionSettings(options Options) (string, string, schema.Dialect, error) {
	switch options.Type {
	case DatabaseSQLite:
		return "sqlite", options.DSN, sqlitedialect.New(), nil
	case DatabaseMySQL:
		config, err := mysqlDriver.ParseDSN(options.DSN)
		if err != nil {
			return "", "", nil, fmt.Errorf("parse mysql DSN: %w", err)
		}
		config.ParseTime = true
		config.Loc = time.UTC
		if config.Params == nil {
			config.Params = map[string]string{}
		}
		if _, ok := config.Params["charset"]; !ok {
			config.Params["charset"] = "utf8mb4"
		}
		return "mysql", config.FormatDSN(), mysqldialect.New(), nil
	default:
		return "", "", nil, fmt.Errorf("unsupported database type %q", options.Type)
	}
}

func configurePool(db *sql.DB, options Options) {
	db.SetMaxOpenConns(options.MaxOpenConns)
	db.SetMaxIdleConns(options.MaxIdleConns)
	db.SetConnMaxLifetime(options.ConnMaxLifetime)
	db.SetConnMaxIdleTime(options.ConnMaxIdleTime)
}

func (s *Store) DatabaseType() DatabaseType { return s.databaseType }
