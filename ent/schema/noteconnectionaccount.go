package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// NoteConnectionAccount stores user-owned external note provider accounts.
type NoteConnectionAccount struct {
	ent.Schema
}

// Fields of the NoteConnectionAccount.
func (NoteConnectionAccount) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			NotEmpty(),
		field.String("provider").
			NotEmpty().
			Comment("siyuan, notion, or joplin"),
		field.Int("user_id"),
		field.String("endpoint").
			Optional().
			Comment("Provider API endpoint for local/self-hosted providers"),
		field.Bool("enabled").
			Default(true),
		field.String("auth_type").
			Default("token"),
		field.Text("encrypted_credentials").
			Optional().
			Sensitive(),
		field.String("credential_alg").
			Optional(),
		field.String("default_target_id").
			Optional().
			Comment("Provider-specific notebook/folder/page id"),
		field.String("default_target_name").
			Optional(),
		field.Text("metadata_json").
			Optional(),
		field.String("last_test_status").
			Default("never").
			Comment("never, success, failed"),
		field.Text("last_test_error").
			Optional(),
		field.Time("last_test_at").
			Optional(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the NoteConnectionAccount.
func (NoteConnectionAccount) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("note_connection_accounts").
			Field("user_id").
			Unique().
			Required(),
		edge.To("item_maps", NoteConnectionItemMap.Type),
		edge.To("jobs", NoteConnectionJob.Type),
	}
}

// Indexes of the NoteConnectionAccount.
func (NoteConnectionAccount) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "provider", "name").
			Unique(),
	}
}
