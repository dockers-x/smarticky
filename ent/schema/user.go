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
		field.String("share_signature").
			Default("Smarticky"),
		field.String("time_zone").
			Default("UTC"),
		field.String("lazycat_uid").
			Optional().
			Nillable().
			Unique(),
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
		edge.To("folders", Folder.Type),
		edge.To("attachments", Attachment.Type),
		edge.To("excalidraw_library", ExcalidrawLibrary.Type).Unique(),
		edge.To("whiteboards", Whiteboard.Type),
		edge.To("tags", Tag.Type),   // User can have many tags
		edge.To("fonts", Font.Type), // User can upload many fonts
		edge.To("import_jobs", ImportJob.Type),
		edge.To("mcp_tokens", MCPToken.Type),
		edge.To("mcp_images", MCPImage.Type),
	}
}
