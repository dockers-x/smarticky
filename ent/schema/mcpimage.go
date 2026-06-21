package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// MCPImage records generated MCP share image files and their owners.
type MCPImage struct {
	ent.Schema
}

// Fields of the MCPImage.
func (MCPImage) Fields() []ent.Field {
	return []ent.Field{
		field.String("filename").
			NotEmpty(),
		field.String("path").
			Sensitive().
			NotEmpty(),
		field.String("content_type").
			Default("image/png"),
		field.Int64("size").
			NonNegative(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the MCPImage.
func (MCPImage) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("mcp_images").
			Unique().
			Required(),
	}
}
