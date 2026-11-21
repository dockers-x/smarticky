package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
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
		field.Int("user_id").StorageKey("user_notes").Optional().Nillable().StructTag(`json:"-"`),
		field.UUID("folder_id", uuid.UUID{}).StorageKey("folder_notes").Optional().Nillable().StructTag(`json:"-"`),
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
			Field("user_id").
			Unique(),
		edge.From("folder", Folder.Type).
			Ref("notes").
			Field("folder_id").
			Unique(),
		edge.To("attachments", Attachment.Type),
		edge.To("whiteboards", Whiteboard.Type),
		edge.To("outgoing_links", NoteLink.Type).
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("backlinks", NoteLink.Type).
			Annotations(entsql.OnDelete(entsql.SetNull)),
		edge.To("connection_maps", NoteConnectionItemMap.Type),
		edge.To("connection_jobs", NoteConnectionJob.Type),
		edge.To("tags", Tag.Type), // Many-to-many relationship with tags
	}
}

func (Note) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "is_deleted", "updated_at"),
		index.Fields("user_id", "is_deleted", "folder_id"),
	}
}
