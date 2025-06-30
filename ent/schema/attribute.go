package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Attribute holds the schema definition for the Attribute entity.
type Attribute struct {
	ent.Schema
}

// Fields of the Attribute.
func (Attribute) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").
			Unique().
			Immutable().
			Comment("Unique identifier for the attribute"),
		field.String("name").
			Unique().
			Immutable().
			Comment("Name of the attribute"),
		field.Enum("type").
			Values("tag", "numeric", "date", "string").
			Immutable().
			Comment("Type of the attribute"),
	}
}

// Edges of the Attribute.
func (Attribute) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("media", Media.Type).
			Ref("tags").
			Through("media_attributes", MediaAttribute.Type),
	}
}
