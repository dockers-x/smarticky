package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type BackupTask struct {
	ent.Schema
}

func (BackupTask) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			NotEmpty(),
		field.Bool("enabled").
			Default(true),
		field.String("schedule").
			Default("manual").
			Comment("manual, daily, weekly, monthly"),
		field.Int("retention_days").
			Default(30).
			Comment("Number of days to retain backup files (0 = no limit)"),
		field.Int("max_count").
			Default(10).
			Comment("Maximum number of backup files to keep (0 = no limit)"),
		field.String("last_backup_status").
			Default("never").
			Comment("never, success, failed"),
		field.Text("last_backup_error").
			Optional(),
		field.Time("last_backup_at").
			Optional(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

func (BackupTask) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("targets", BackupTarget.Type),
	}
}
