package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// MediaAttribute is the join table between Media and Attribute with an optional value.
type MediaAttribute struct {
	ent.Schema
}

func (MediaAttribute) Annotations() []schema.Annotation {
	return []schema.Annotation{
		field.ID("media_id", "attribute_id"),
	}
}

// Fields of the MediaAttribute.
func (MediaAttribute) Fields() []ent.Field {
	return []ent.Field{
		field.String("media_id"),
		field.Int("attribute_id"),
		field.String("value").
			Optional().
			Nillable().
			Comment("Value for the attribute; null for tag type"),
	}
}

// Edges of the MediaAttribute.
func (MediaAttribute) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("media", Media.Type).
			Required().
			Unique().
			Field("media_id"),
		edge.To("attribute", Attribute.Type).
			Required().
			Unique().
			Field("attribute_id"),
	}
}
