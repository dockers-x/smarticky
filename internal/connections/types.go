package connections

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const (
	ProviderSiYuan = "siyuan"
	ProviderNotion = "notion"
	ProviderJoplin = "joplin"

	StatusNever   = "never"
	StatusSuccess = "success"
	StatusFailed  = "failed"

	OperationImport = "import"
	OperationPush   = "push"

	JobPending             = "pending"
	JobRunning             = "running"
	JobCompleted           = "completed"
	JobCompletedWithErrors = "completed_with_errors"
	JobFailed              = "failed"
)

type Credentials struct {
	Token string `json:"token"`
}

type AccountInput struct {
	Name              string  `json:"name"`
	Provider          string  `json:"provider"`
	Endpoint          string  `json:"endpoint"`
	Token             *string `json:"token,omitempty"`
	DefaultTargetID   string  `json:"default_target_id"`
	DefaultTargetName string  `json:"default_target_name"`
	Enabled           bool    `json:"enabled"`
	ClearCredentials  bool    `json:"clear_credentials"`
}

type AccountResponse struct {
	ID                int        `json:"id"`
	Name              string     `json:"name"`
	Provider          string     `json:"provider"`
	Endpoint          string     `json:"endpoint"`
	Enabled           bool       `json:"enabled"`
	AuthType          string     `json:"auth_type"`
	HasCredentials    bool       `json:"has_credentials"`
	DefaultTargetID   string     `json:"default_target_id"`
	DefaultTargetName string     `json:"default_target_name"`
	LastTestStatus    string     `json:"last_test_status"`
	LastTestError     string     `json:"last_test_error,omitempty"`
	LastTestAt        *time.Time `json:"last_test_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type Target struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	ParentID string `json:"parent_id,omitempty"`
}

type RemoteNote struct {
	ExternalID string
	TargetID   string
	TargetName string
	Path       string
	URL        string
	Title      string
	Content    string
	CreatedAt  *time.Time
	UpdatedAt  *time.Time
}

type PushInput struct {
	NoteID             uuid.UUID
	Title              string
	Content            string
	TargetID           string
	ExistingExternalID string
}

type PushResult struct {
	ExternalID string `json:"external_id"`
	TargetID   string `json:"target_id,omitempty"`
	Path       string `json:"path,omitempty"`
	URL        string `json:"url,omitempty"`
}

type ImportRequest struct {
	TargetID string `json:"target_id"`
	Limit    int    `json:"limit"`
}

type ImportResult struct {
	JobID         int    `json:"job_id"`
	Status        string `json:"status"`
	TotalCount    int    `json:"total_count"`
	ImportedCount int    `json:"imported_count"`
	SkippedCount  int    `json:"skipped_count"`
	FailedCount   int    `json:"failed_count"`
}

type PushRequest struct {
	AccountID int    `json:"account_id"`
	TargetID  string `json:"target_id"`
}

type PushResponse struct {
	JobID      int        `json:"job_id"`
	Status     string     `json:"status"`
	Result     PushResult `json:"result"`
	FinishedAt time.Time  `json:"finished_at"`
}

type Provider interface {
	Test(ctx context.Context) error
	ListTargets(ctx context.Context) ([]Target, error)
	ImportNotes(ctx context.Context, targetID string, limit int) ([]RemoteNote, error)
	PushNote(ctx context.Context, input PushInput) (PushResult, error)
}
