package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// NoteLink holds deterministic note-to-note backlink relationships.
type NoteLink struct {
	ent.Schema
}

// Fields of the NoteLink.
func (NoteLink) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.Int("user_id"),
		field.UUID("source_note_id", uuid.UUID{}),
		field.UUID("target_note_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.String("target_ref"),
		field.String("target_ref_norm"),
		field.String("target_key"),
		field.String("display_text"),
		field.Enum("link_type").
			Values("wiki").
			Default("wiki"),
		field.Int("occurrence_count").
			Default(1),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the NoteLink.
func (NoteLink) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("note_links").
			Field("user_id").
			Unique().
			Required(),
		edge.From("source_note", Note.Type).
			Ref("outgoing_links").
			Field("source_note_id").
			Unique().
			Required(),
		edge.From("target_note", Note.Type).
			Ref("backlinks").
			Field("target_note_id").
			Unique(),
	}
}

// Indexes of the NoteLink.
func (NoteLink) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("source_note_id", "target_key", "link_type").
			Unique(),
		index.Fields("user_id", "source_note_id"),
		index.Fields("user_id", "target_note_id"),
		index.Fields("user_id", "target_ref_norm"),
	}
}
