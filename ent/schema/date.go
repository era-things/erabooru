package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Date holds the schema definition for the Date entity.
type Date struct {
	ent.Schema
}

func (Date) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").
			Unique().
			Immutable().
			Comment("Unique identifier for the date name"),
		field.String("name").
			Unique().
			Immutable().
			Comment("Name of the date entry"),
	}
}

func (Date) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("media", Media.Type).
			Ref("dates").
			Through("media_dates", MediaDate.Type).
			Comment("Media items associated with this date"),
	}
}
