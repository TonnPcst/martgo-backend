package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upSeedAdmin, downSeedAdmin)
}

// Admin seeding moved to the auth service (martgo-auth: migrations/00002_seed_admin.go).
// This version is kept as a no-op so goose's history stays intact.
func upSeedAdmin(ctx context.Context, tx *sql.Tx) error   { return nil }
func downSeedAdmin(ctx context.Context, tx *sql.Tx) error { return nil }
