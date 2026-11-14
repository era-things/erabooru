package tagmodel

import (
	"context"
	"math"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestScoreSQL(t *testing.T) {
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
	score := (ScoreSQL{
		Value: "$1::vector", ValueNorm: "vector_norm($1::vector)",
		Positive: "$2::vector", PositiveNorm: "vector_norm($2::vector)",
		Negative: "$3::vector", NegativeNorm: "vector_norm($3::vector)", NegativeCount: "$4::bigint",
		Classifier: "$5::boolean", Weights: "$6::vector", Bias: "$7::float8",
	}).Expression()
	for _, test := range []struct {
		name, value, positive, negative string
		negatives                       int
		classifier                      bool
		want                            float64
		null                            bool
	}{
		{"normalized classifier", "[3,4]", "[0,0]", "[0,0]", 0, true, 1.45, false},
		{"centroid", "[1,0]", "[1,0]", "[0,0]", 0, false, 1, false},
		{"rejection penalty", "[1,0]", "[1,0]", "[1,0]", 1, false, 0.5, false},
		{"orthogonal negative", "[1,0]", "[1,0]", "[0,1]", 1, false, 1, false},
		{"no usable model", "[1,0]", "[0,0]", "[0,0]", 0, false, 0, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var got *float64
			if err := pool.QueryRow(ctx, "SELECT "+score, test.value, test.positive, test.negative, test.negatives, test.classifier, "[2,0]", 0.25).Scan(&got); err != nil {
				t.Fatal(err)
			}
			if test.null {
				if got != nil {
					t.Fatalf("expected null score, got %v", *got)
				}
				return
			}
			if got == nil || math.Abs(*got-test.want) > 1e-12 {
				t.Fatalf("score=%v, want=%v", got, test.want)
			}
		})
	}
}
