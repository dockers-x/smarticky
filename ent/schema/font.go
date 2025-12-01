package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Font holds the schema definition for the Font entity.
type Font struct {
	ent.Schema
}

// Fields of the Font.
func (Font) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			StorageKey("id"),
		field.String("name").
			NotEmpty().
			Comment("Font family name (e.g., 'MyCustomFont')"),
		field.String("display_name").
			NotEmpty().
			Comment("Display name shown to users"),
		field.String("file_path").
			NotEmpty().
			Comment("Path to the font file"),
		field.Int64("file_size").
			Positive().
			Comment("File size in bytes"),
		field.Enum("format").
			Values("ttf", "otf", "woff", "woff2").
			Comment("Font file format"),
		field.String("preview_text").
			Default("The quick brown fox jumps over the lazy dog 我能吞下玻璃而不伤身体").
			Comment("Text used for font preview"),
		field.Bool("is_shared").
			Default(true).
			Comment("Whether the font is shared with all users"),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Comment("Upload time"),
	}
}

// Edges of the Font.
func (Font) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("uploaded_by", User.Type).
			Ref("fonts").
			Unique().
			Required().
			Comment("User who uploaded this font"),
	}
}
