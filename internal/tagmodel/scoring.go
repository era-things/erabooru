package tagmodel

import "strings"

// ScoreSQL binds SQL expressions supplied by our queries, never user input.
// Callers may supply stored/precomputed norms without changing scoring rules.
type ScoreSQL struct {
	Value, ValueNorm                      string
	Positive, PositiveNorm                string
	Negative, NegativeNorm, NegativeCount string
	Classifier, Weights, Bias             string
}

func (s ScoreSQL) Expression() string {
	return strings.NewReplacer(
		"{value}", s.Value, "{norm}", s.ValueNorm,
		"{positive}", s.Positive, "{positive_norm}", s.PositiveNorm,
		"{negative}", s.Negative, "{negative_norm}", s.NegativeNorm, "{negative_count}", s.NegativeCount,
		"{classifier}", s.Classifier, "{weights}", s.Weights, "{bias}", s.Bias,
	).Replace(`CASE WHEN {classifier} THEN {bias}-({value} <#> {weights})/{norm}
 WHEN {positive_norm}>0 THEN 1-({value} <=> {positive}) -
  0.5 * CASE WHEN {negative_count}>0 AND {negative_norm}>0 THEN 1-({value} <=> {negative}) ELSE 0 END
 END`)
}
