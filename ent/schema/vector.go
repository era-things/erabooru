package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Vector holds the schema definition for the Vector entity.
type Vector struct {
	ent.Schema
}

func (Vector) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").
			Unique().
			Immutable().
			Comment("Unique identifier for the vector name"),
		field.String("name").
			Unique().
			Immutable().
			Comment("Name of the vector entry"),
	}
}

func (Vector) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("media", Media.Type).
			Ref("vectors").
			Through("media_vectors", MediaVector.Type).
			Comment("Media items associated with this vector"),
	}
}
