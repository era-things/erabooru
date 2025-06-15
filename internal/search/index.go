package search

import (
	"fmt"
	"os"
	"strconv"

	"era/booru/ent"
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

// IndexMedia indexes the media metadata in the Bleve index.
func IndexMedia(m *ent.Media) error {
	if IDX == nil {
		return fmt.Errorf("index not open")
	}
	return IDX.Index(strconv.Itoa(m.ID), m)
}

// DeleteMedia removes the document from the Bleve index.
func DeleteMedia(id int) error {
	if IDX == nil {
		return fmt.Errorf("index not open")
	}
	return IDX.Delete(strconv.Itoa(id))
}
