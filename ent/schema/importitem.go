package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type ImportItem struct {
	ent.Schema
}

func (ImportItem) Fields() []ent.Field {
	return []ent.Field{
		field.String("source_note_key").NotEmpty(),
		field.UUID("note_id", uuid.UUID{}).Optional(),
		field.String("title").Default("Untitled"),
		field.String("status").Default("pending"),
		field.Text("message").Optional(),
	}
}

func (ImportItem) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("job", ImportJob.Type).Ref("items").Unique().Required(),
	}
}
