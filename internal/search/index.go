package search

import (
	"os"

	"github.com/blevesearch/bleve/v2"
)

var IDX bleve.Index // global handle

// OpenOrCreate initialises the index at start-up.
func OpenOrCreate(path string) error {
	var err error
	if _, err = os.Stat(path); os.IsNotExist(err) {
		mapping := bleve.NewIndexMapping() // tweak if you need custom analysers
		IDX, err = bleve.New(path, mapping)
	} else {
		IDX, err = bleve.Open(path)
	}
	return err
}
