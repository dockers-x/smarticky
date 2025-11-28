package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Attachment holds the schema definition for the Attachment entity.
type Attachment struct {
	ent.Schema
}

// Fields of the Attachment.
func (Attachment) Fields() []ent.Field {
	return []ent.Field{
		field.String("filename").
			NotEmpty(),
		field.String("file_path").
			NotEmpty(),
		field.Int64("file_size").
			Default(0),
		field.String("mime_type").
			Optional(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

// Edges of the Attachment.
func (Attachment) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("note", Note.Type).
			Ref("attachments").
			Unique().
			Required(),
		edge.From("user", User.Type).
			Ref("attachments").
			Unique(),
	}
}
