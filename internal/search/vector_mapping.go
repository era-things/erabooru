package search

import "github.com/blevesearch/bleve/v2/mapping"

// configureVectorMapping applies vector field configuration when available.
// In the current build this is a no-op, but the hook allows vector-enabled
// builds to provide their own implementation.
func configureVectorMapping(m mapping.IndexMapping) {}
