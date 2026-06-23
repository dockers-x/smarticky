package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type BackupTarget struct {
	ent.Schema
}

func (BackupTarget) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			NotEmpty(),
		field.String("type").
			NotEmpty().
			Comment("webdav or s3"),
		field.Bool("enabled").
			Default(true),
		field.String("last_backup_status").
			Default("never").
			Comment("never, success, failed"),
		field.Text("last_backup_error").
			Optional(),
		field.Time("last_backup_at").
			Optional(),
		field.String("last_test_status").
			Default("never").
			Comment("never, success, failed"),
		field.Text("last_test_error").
			Optional(),
		field.Time("last_test_at").
			Optional(),
		field.String("webdav_url").
			Optional(),
		field.String("webdav_user").
			Optional(),
		field.String("webdav_password").
			Optional().
			Sensitive(),
		field.String("s3_endpoint").
			Optional(),
		field.String("s3_region").
			Optional(),
		field.String("s3_bucket").
			Optional(),
		field.String("s3_access_key").
			Optional().
			Sensitive(),
		field.String("s3_secret_key").
			Optional().
			Sensitive(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

func (BackupTarget) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tasks", BackupTask.Type).Ref("targets"),
	}
}
