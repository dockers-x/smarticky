package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Note holds the schema definition for the Note entity.
type Note struct {
	ent.Schema
}

// Fields of the Note.
func (Note) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.String("title").
			Default("Untitled"),
		field.Text("content").
			Optional(),
		field.String("color").
			Optional().
			Default(""), // yellow, green, blue, pink, purple, etc.
		field.Enum("protection_mode").
			Values("none", "password", "encrypted").
			Default("none"),
		field.String("protection_password_hash").
			Optional().
			Sensitive(),
		field.Text("encrypted_content").
			Optional().
			Sensitive(),
		field.String("encryption_alg").
			Optional(),
		field.String("encryption_kdf").
			Optional(),
		field.String("encryption_salt").
			Optional(),
		field.String("encryption_nonce").
			Optional(),
		field.Bool("is_starred").
			Default(false),
		field.Bool("is_deleted").
			Default(false), // For trash bin
		field.Time("created_at").
			Default(time.Now),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the Note.
func (Note) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("notes").
			Unique(),
		edge.From("folder", Folder.Type).
			Ref("notes").
			Unique(),
		edge.To("attachments", Attachment.Type),
		edge.To("whiteboards", Whiteboard.Type),
		edge.To("tags", Tag.Type), // Many-to-many relationship with tags
	}
}
