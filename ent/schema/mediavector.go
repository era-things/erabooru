package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	pgvector "github.com/pgvector/pgvector-go"
)

// MediaVector is the join table between Media and Vector with a value field.
type MediaVector struct {
	ent.Schema
}

func (MediaVector) Fields() []ent.Field {
	return []ent.Field{
		field.String("media_id"),
		field.Int("vector_id"),
		field.Other("value", pgvector.Vector{}).
			SchemaType(map[string]string{dialect.Postgres: "vector"}).
			Comment("Vector value for the media/vector relation"),
	}
}

func (MediaVector) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("media", Media.Type).
			Field("media_id").
			Unique().
			Required().
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("vector", Vector.Type).
			Field("vector_id").
			Unique().
			Required(),
	}
}

func (MediaVector) Indexes() []ent.Index {
	return []ent.Index{
		index.Edges("media", "vector").
			Unique(),
	}
}
