// Code generated by ent, DO NOT EDIT.

package ent

import (
	"era/booru/ent/media"
	"era/booru/ent/schema"
)

// The init function reads all schema descriptors with runtime code
// (default values, validators, hooks and policies) and stitches it
// to their package variables.
func init() {
	mediaFields := schema.Media{}.Fields()
	_ = mediaFields
	// mediaDescID is the schema descriptor for id field.
	mediaDescID := mediaFields[0].Descriptor()
	// media.IDValidator is a validator for the "id" field. It is called by the builders before save.
	media.IDValidator = mediaDescID.Validators[0].(func(string) error)
}
