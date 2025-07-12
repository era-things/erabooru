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
		field.String("id").
			NotEmpty().
			Immutable().
			Unique().
			Comment("xxhash128 hash of the file, used as a unique identifier"),
		field.String("format").
			Immutable().
			Comment("File format such as png or jpg"),
		field.Int16("width").
			Immutable().
			Comment("Image width in pixels"),
		field.Int16("height").
			Immutable().
			Comment("Image height in pixels"),
		field.Int16("duration").
			Optional().
			Nillable().
			Comment("Duration in seconds for video or audio"),
	}
}

// Edges of the Media.
func (Media) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("tags", Tag.Type).
			Comment("Tags associated with the media item, used for categorization"),
		edge.To("dates", Date.Type).
			Through("media_dates", MediaDate.Type).
			Comment("Date entries associated with the media item"),
		edge.To("vectors", Vector.Type).
			Through("media_vectors", MediaVector.Type).
			Comment("Vector entries associated with the media item"),
	}
}
