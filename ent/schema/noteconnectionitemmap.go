package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// NoteConnectionItemMap links a local note with an external provider item.
type NoteConnectionItemMap struct {
	ent.Schema
}

// Fields of the NoteConnectionItemMap.
func (NoteConnectionItemMap) Fields() []ent.Field {
	return []ent.Field{
		field.String("provider").
			NotEmpty(),
		field.String("external_id").
			NotEmpty(),
		field.Int("account_id"),
		field.UUID("note_id", uuid.UUID{}),
		field.String("external_target_id").
			Optional(),
		field.String("external_path").
			Optional(),
		field.String("external_url").
			Optional(),
		field.String("last_sync_direction").
			Optional().
			Comment("import or push"),
		field.Time("last_imported_at").
			Optional(),
		field.Time("last_pushed_at").
			Optional(),
		field.Text("metadata_json").
			Optional(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the NoteConnectionItemMap.
func (NoteConnectionItemMap) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("account", NoteConnectionAccount.Type).
			Ref("item_maps").
			Field("account_id").
			Unique().
			Required(),
		edge.From("note", Note.Type).
			Ref("connection_maps").
			Field("note_id").
			Unique().
			Required(),
	}
}

// Indexes of the NoteConnectionItemMap.
func (NoteConnectionItemMap) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("account_id", "external_id").
			Unique(),
		index.Fields("note_id", "account_id").
			Unique(),
	}
}
