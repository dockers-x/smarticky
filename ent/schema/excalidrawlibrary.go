package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// ExcalidrawLibrary holds the user-scoped Excalidraw component library.
type ExcalidrawLibrary struct {
	ent.Schema
}

// Fields of the ExcalidrawLibrary.
func (ExcalidrawLibrary) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.Text("library_json").
			Default("[]"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the ExcalidrawLibrary.
func (ExcalidrawLibrary) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("excalidraw_library").
			Unique().
			Required(),
	}
}
