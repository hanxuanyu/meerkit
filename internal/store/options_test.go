package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"strings"
	"testing"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
)

func TestNormalizeSQLiteOptions(t *testing.T) {
	options, err := normalizeOptions(Options{Type: DatabaseSQLite, DataDir: t.TempDir(), AutoMigrate: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(options.DSN, "meerkit.db") || !strings.Contains(options.DSN, "journal_mode(WAL)") {
		t.Fatalf("unexpected sqlite DSN: %s", options.DSN)
	}
	if options.MaxOpenConns != 4 || options.MaxIdleConns != 4 {
		t.Fatalf("unexpected sqlite pool: %+v", options)
	}
}

func TestMySQLSchemaGeneration(t *testing.T) {
	sqlDB := sql.OpenDB(staticMySQLConnector{})
	defer sqlDB.Close()
	db := bun.NewDB(sqlDB, mysqldialect.New())
	defer db.Close()

	recordDDL := db.NewCreateTable().Model((*monitorRecordModel)(nil)).WithForeignKeys().String()
	for _, fragment := range []string{"CREATE TABLE", "`monitor_records`", "longtext", "FOREIGN KEY", "ON DELETE CASCADE"} {
		if !strings.Contains(recordDDL, fragment) {
			t.Fatalf("mysql record DDL is missing %q: %s", fragment, recordDDL)
		}
	}
	deliveryDDL := db.NewCreateTable().Model((*notificationDeliveryModel)(nil)).String()
	if !strings.Contains(deliveryDDL, "`notification_deliveries`") || !strings.Contains(deliveryDDL, "`payload_json` longtext") {
		t.Fatalf("unexpected mysql delivery DDL: %s", deliveryDDL)
	}
}

type staticMySQLConnector struct{}

func (staticMySQLConnector) Connect(context.Context) (driver.Conn, error) {
	return staticMySQLConnection{}, nil
}

func (staticMySQLConnector) Driver() driver.Driver { return staticMySQLDriver{} }

type staticMySQLDriver struct{}

func (staticMySQLDriver) Open(string) (driver.Conn, error) { return staticMySQLConnection{}, nil }

type staticMySQLConnection struct{}

func (staticMySQLConnection) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (staticMySQLConnection) Close() error                        { return nil }
func (staticMySQLConnection) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }
func (staticMySQLConnection) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &staticMySQLRows{}, nil
}

type staticMySQLRows struct{ returned bool }

func (*staticMySQLRows) Columns() []string { return []string{"version()"} }
func (*staticMySQLRows) Close() error      { return nil }
func (rows *staticMySQLRows) Next(values []driver.Value) error {
	if rows.returned {
		return io.EOF
	}
	rows.returned = true
	values[0] = "8.0.36"
	return nil
}

func TestMySQLConnectionSettingsEnforcePortableDefaults(t *testing.T) {
	options, err := normalizeOptions(Options{Type: DatabaseMySQL, DSN: "user:secret@tcp(localhost:3306)/meerkit"})
	if err != nil {
		t.Fatal(err)
	}
	driver, dsn, _, err := connectionSettings(options)
	if err != nil {
		t.Fatal(err)
	}
	if driver != "mysql" || !strings.Contains(dsn, "parseTime=true") || !strings.Contains(dsn, "charset=utf8mb4") {
		t.Fatalf("unexpected mysql settings: driver=%s dsn=%s", driver, dsn)
	}
	if options.MaxOpenConns != 25 || options.MaxIdleConns != 10 {
		t.Fatalf("unexpected mysql pool: %+v", options)
	}
}

func TestMySQLRejectsSingleConnectionPool(t *testing.T) {
	_, err := normalizeOptions(Options{Type: DatabaseMySQL, DSN: "user:secret@tcp(localhost:3306)/meerkit", MaxOpenConns: 1})
	if err == nil || !strings.Contains(err.Error(), "at least 2") {
		t.Fatalf("expected mysql pool validation error, got %v", err)
	}
}

func TestOpenWithoutAutoMigrationRequiresSchema(t *testing.T) {
	_, err := Open(context.Background(), Options{Type: DatabaseSQLite, DataDir: t.TempDir(), AutoMigrate: false})
	if err == nil || !strings.Contains(err.Error(), "schema") {
		t.Fatalf("expected schema validation error, got %v", err)
	}
}
