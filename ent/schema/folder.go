package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Folder holds the schema definition for the Folder entity.
type Folder struct {
	ent.Schema
}

// Fields of the Folder.
func (Folder) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.String("name").
			NotEmpty(),
		field.Int("sort_order").
			Default(0),
		field.Bool("is_starred").
			Default(false),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the Folder.
func (Folder) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("folders").
			Unique().
			Required(),
		edge.To("children", Folder.Type).
			From("parent").
			Unique(),
		edge.To("notes", Note.Type),
	}
}
