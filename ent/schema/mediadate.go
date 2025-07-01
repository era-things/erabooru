package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// MediaDate is the join table between Media and Date with a value field.
type MediaDate struct {
	ent.Schema
}

func (MediaDate) Fields() []ent.Field {
	return []ent.Field{
		field.String("media_id"),
		field.Int("date_id"),
		field.Time("value").
			SchemaType(map[string]string{dialect.Postgres: "date"}).
			Comment("Date value for the media/date relation"),
	}
}

func (MediaDate) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("media", Media.Type).
			Field("media_id").
			Unique().
			Required(),
		edge.To("date", Date.Type).
			Field("date_id").
			Unique().
			Required(),
	}
}

func (MediaDate) Indexes() []ent.Index {
	return []ent.Index{
		index.Edges("media", "date").
			Unique(),
	}
}
