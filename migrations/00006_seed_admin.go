package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

// init registers the seed-admin Go migration with goose.
//
// ENG: Goose picks up Go migrations through init, using the file name for the version.
// LAO: ລົງທະບຽນ migration seed admin ກັບ goose.
func init() {
	goose.AddMigrationContext(upSeedAdmin, downSeedAdmin)
}

// upSeedAdmin is a no-op kept so goose's history stays intact.
//
// ENG: Admin seeding moved to the auth service (martgo-auth: migrations/00002_seed_admin.go).
// LAO: ບໍ່ເຮັດຫຍັງ — ການສ້າງ admin ຍ້າຍໄປ auth service ແລ້ວ, ເກັບໄວ້ເພື່ອໃຫ້ປະຫວັດ goose ຄົບ.
//
// @param ctx - migration context
// @param tx - migration transaction
// @returns always nil
func upSeedAdmin(ctx context.Context, tx *sql.Tx) error { return nil }

// downSeedAdmin is a no-op, matching upSeedAdmin.
//
// ENG: Nothing to undo, since upSeedAdmin does nothing.
// LAO: ບໍ່ເຮັດຫຍັງ — ຄູ່ກັບ upSeedAdmin.
//
// @param ctx - migration context
// @param tx - migration transaction
// @returns always nil
func downSeedAdmin(ctx context.Context, tx *sql.Tx) error { return nil }
