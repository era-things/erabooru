package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Media holds the schema definition for the Media entity.
type Media struct {
	ent.Schema
}

// Fields of the Media.
func (Media) Fields() []ent.Field {
	return []ent.Field{
		field.String("hash").
			Unique().
			Immutable().
			Comment("Hash of the media file, used for deduplication"),
		field.Enum("type").
			Values("image", "video", "audio").
			Immutable().
			Comment("Type of the media, can be image, video, or audio"),
	}
}

// Edges of the Media.
func (Media) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("tags", Tag.Type).
			Comment("Tags associated with the media item, used for categorization"),
	}
}
