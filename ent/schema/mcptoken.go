package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// MCPToken holds user-bound MCP access tokens for non-LazyCat clients.
type MCPToken struct {
	ent.Schema
}

// Fields of the MCPToken.
func (MCPToken) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Default("MCP Token"),
		field.String("token_hash").
			Unique().
			Sensitive().
			NotEmpty(),
		field.Time("last_used_at").
			Optional().
			Nillable(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the MCPToken.
func (MCPToken) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("mcp_tokens").
			Unique().
			Required(),
	}
}
