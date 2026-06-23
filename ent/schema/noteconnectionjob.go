package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// NoteConnectionJob records external note import and push operations.
type NoteConnectionJob struct {
	ent.Schema
}

// Fields of the NoteConnectionJob.
func (NoteConnectionJob) Fields() []ent.Field {
	return []ent.Field{
		field.String("provider").
			NotEmpty(),
		field.String("operation").
			NotEmpty().
			Comment("import or push"),
		field.Int("user_id"),
		field.Int("account_id"),
		field.UUID("note_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.String("status").
			Default("pending").
			Comment("pending, running, completed, completed_with_errors, failed"),
		field.Int("total_count").
			Default(0),
		field.Int("imported_count").
			Default(0),
		field.Int("pushed_count").
			Default(0),
		field.Int("skipped_count").
			Default(0),
		field.Int("failed_count").
			Default(0),
		field.Text("message").
			Optional(),
		field.Text("options_json").
			Optional(),
		field.Time("started_at").
			Optional(),
		field.Time("completed_at").
			Optional(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the NoteConnectionJob.
func (NoteConnectionJob) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("note_connection_jobs").
			Field("user_id").
			Unique().
			Required(),
		edge.From("account", NoteConnectionAccount.Type).
			Ref("jobs").
			Field("account_id").
			Unique().
			Required(),
		edge.From("note", Note.Type).
			Ref("connection_jobs").
			Field("note_id").
			Unique(),
	}
}
