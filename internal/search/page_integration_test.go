package search

import (
	"context"
	"os"
	"testing"

	"era/booru/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCollectMediaPage(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_DSN to an isolated test database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for _, test := range []struct {
		name             string
		count, wantCount int
		wantNext         bool
	}{
		{"empty", 0, 0, false},
		{"exact page", 2, 2, false},
		{"extra row", 3, 2, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Equal distances exercise the ID tie-break cursor; the extra row
			// must never become the cursor for the returned page.
			rows, err := pool.Query(ctx, `SELECT i::text,0::float8 FROM generate_series(1,$1::int) i ORDER BY i`, test.count)
			if err != nil {
				t.Fatal(err)
			}
			items, next, err := collectMediaPage(rows, 2, func(row pgx.Row, m *db.Media) (vectorCursor, error) {
				var distance float64
				err := row.Scan(&m.ID, &distance)
				return vectorCursor{Distance: distance, ID: m.ID}, err
			})
			if err != nil || len(items) != test.wantCount || (next != "") != test.wantNext {
				t.Fatalf("items=%+v next=%q err=%v", items, next, err)
			}
			if test.wantNext {
				cursor, err := decodeCursor[vectorCursor](next)
				if err != nil || cursor.ID != "2" || cursor.Distance != 0 {
					t.Fatalf("incorrect cursor: %+v, err=%v", cursor, err)
				}
			}
		})
	}
}
