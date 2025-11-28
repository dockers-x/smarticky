package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// BackupConfig holds the schema definition for backup configuration.
type BackupConfig struct {
	ent.Schema
}

// Fields of the BackupConfig.
func (BackupConfig) Fields() []ent.Field {
	return []ent.Field{
		field.String("webdav_url").
			Optional(),
		field.String("webdav_user").
			Optional(),
		field.String("webdav_password").
			Optional().
			Sensitive(), // Don't log this field
		field.String("s3_endpoint").
			Optional(),
		field.String("s3_region").
			Optional(),
		field.String("s3_bucket").
			Optional(),
		field.String("s3_access_key").
			Optional(),
		field.String("s3_secret_key").
			Optional().
			Sensitive(),
		field.Bool("auto_backup_enabled").
			Default(false),
		field.String("backup_schedule").
			Default("daily"), // daily, weekly, manual
		field.Time("last_backup_at").
			Optional(),
		field.Time("created_at").
			Default(time.Now),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the BackupConfig.
func (BackupConfig) Edges() []ent.Edge {
	return nil
}
