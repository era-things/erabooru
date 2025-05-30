package schema

import "entgo.io/ent"

// Media holds the schema definition for the Media entity.
type Media struct {
	ent.Schema
}

// Fields of the Media.
func (Media) Fields() []ent.Field {
	return nil
}

// Edges of the Media.
func (Media) Edges() []ent.Edge {
	return nil
}
