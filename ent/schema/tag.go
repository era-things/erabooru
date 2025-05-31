package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Tag holds the schema definition for the Tag entity.
type Tag struct {
	ent.Schema
}

// Fields of the Tag.
func (Tag) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").
			Unique().
			Immutable().
			Comment("Unique identifier for the tag"),
		field.String("name").
			Unique().
			Immutable().
			Comment("Name of the tag, used for categorization"),
		field.Enum("type").
			Values("user_tag", "meta_tag").
			Immutable().
			Comment("Type of the tag, can be user_tag or meta_tag"),
	}
}

// Edges of the Tag.
func (Tag) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("media", Media.Type).
			Ref("tags").
			Comment("Media items associated with this tag, used for categorization"),
	}
}
