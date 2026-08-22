package store

import (
	"context"
	"testing"
)

func TestSchemaMigrationsKeepTokenSchemaInV1(t *testing.T) {
	database, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	migrations := database.schemaMigrations()
	if len(migrations) != 1 || migrations[0].version != 1 {
		t.Fatalf("expected a single v1 migration, got %+v", migrations)
	}
	if migrations[0].name != "initial cross-database schema" {
		t.Fatalf("unexpected initial migration name: %q", migrations[0].name)
	}

	count, err := database.orm.NewSelect().Model((*apiTokenModel)(nil)).Count(context.Background())
	if err != nil {
		t.Fatalf("api_tokens table is not available from v1 schema: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected empty api_tokens table, got %d rows", count)
	}
}
