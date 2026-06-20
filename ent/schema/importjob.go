package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type ImportJob struct {
	ent.Schema
}

func (ImportJob) Fields() []ent.Field {
	return []ent.Field{
		field.String("source").Default("evernote"),
		field.String("filename").NotEmpty(),
		field.String("status").Default("previewed"),
		field.Int("note_count").Default(0),
		field.Int("imported_count").Default(0),
		field.Int("skipped_count").Default(0),
		field.Int("failed_count").Default(0),
		field.Text("options_json").Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("completed_at").Optional(),
	}
}

func (ImportJob) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("import_jobs").Unique(),
		edge.To("items", ImportItem.Type),
	}
}
