package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Whiteboard holds the schema definition for an Excalidraw scene attached to a note.
type Whiteboard struct {
	ent.Schema
}

// Fields of the Whiteboard.
func (Whiteboard) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.String("title").
			Default("Whiteboard"),
		field.Text("scene_json").
			Default("{}"),
		field.Text("thumbnail").
			Optional(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the Whiteboard.
func (Whiteboard) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("note", Note.Type).
			Ref("whiteboards").
			Unique().
			Required(),
		edge.From("user", User.Type).
			Ref("whiteboards").
			Unique().
			Required(),
	}
}
