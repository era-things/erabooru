package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// HiddenTagFilter holds the schema definition for the HiddenTagFilter entity.
type HiddenTagFilter struct {
	ent.Schema
}

// Fields of the HiddenTagFilter.
func (HiddenTagFilter) Fields() []ent.Field {
	return []ent.Field{
		field.String("value").
			MaxLen(1024).
			Unique(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the HiddenTagFilter.
func (HiddenTagFilter) Edges() []ent.Edge {
	return nil
}
