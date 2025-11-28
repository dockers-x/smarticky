package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("username").
			Unique().
			NotEmpty(),
		field.String("password_hash").
			Sensitive().
			NotEmpty(),
		field.String("email").
			Optional(),
		field.String("nickname").
			Optional().
			Default(""),
		field.Enum("role").
			Values("admin", "user").
			Default("user"),
		field.String("avatar").
			Optional().
			Default(""),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("notes", Note.Type),
		edge.To("attachments", Attachment.Type),
	}
}
