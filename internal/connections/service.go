package connections

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"smarticky/ent"
	"smarticky/ent/folder"
	"smarticky/ent/note"
	"smarticky/ent/noteconnectionaccount"
	"smarticky/ent/noteconnectionitemmap"
	"smarticky/ent/noteconnectionjob"
	"smarticky/ent/user"
	"smarticky/internal/secrets"

	"github.com/google/uuid"
)

const (
	credentialAlg = "AES-GCM:v1"
	defaultLimit  = 50
	maxLimit      = 200
)

var (
	ErrUnsupportedProvider = errors.New("unsupported note provider")
	ErrMissingCredential   = errors.New("missing provider credential")
	ErrMissingTarget       = errors.New("missing target")
	ErrAccountDisabled     = errors.New("note connection account is disabled")
)

var sensitiveQueryValuePattern = regexp.MustCompile(`(?i)([?&](?:token|access_token|api_key|apikey|password|secret)=)([^&\s"']+)`)

type Service struct {
	client *ent.Client
	box    *secrets.Box
	http   *http.Client
}

func NewService(client *ent.Client, box *secrets.Box) *Service {
	return &Service{
		client: client,
		box:    box,
		http: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (s *Service) ListAccounts(ctx context.Context, userID int) ([]AccountResponse, error) {
	rows, err := s.client.NoteConnectionAccount.Query().
		Where(noteconnectionaccount.UserIDEQ(userID)).
		Order(ent.Asc(noteconnectionaccount.FieldProvider), ent.Asc(noteconnectionaccount.FieldName)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AccountResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, accountResponse(row))
	}
	return out, nil
}

func (s *Service) CreateAccount(ctx context.Context, userID int, input AccountInput) (AccountResponse, error) {
	normalized, err := normalizeAccountInput(input)
	if err != nil {
		return AccountResponse{}, err
	}
	if normalized.Token == nil || strings.TrimSpace(*normalized.Token) == "" {
		return AccountResponse{}, ErrMissingCredential
	}

	encrypted, err := s.encryptCredentials(Credentials{Token: strings.TrimSpace(*normalized.Token)})
	if err != nil {
		return AccountResponse{}, err
	}

	row, err := s.client.NoteConnectionAccount.Create().
		SetName(normalized.Name).
		SetProvider(normalized.Provider).
		SetUserID(userID).
		SetEndpoint(normalized.Endpoint).
		SetEnabled(normalized.Enabled).
		SetAuthType("token").
		SetEncryptedCredentials(encrypted).
		SetCredentialAlg(credentialAlg).
		SetDefaultTargetID(normalized.DefaultTargetID).
		SetDefaultTargetName(normalized.DefaultTargetName).
		Save(ctx)
	if err != nil {
		return AccountResponse{}, err
	}
	return accountResponse(row), nil
}

func (s *Service) UpdateAccount(ctx context.Context, userID, accountID int, input AccountInput) (AccountResponse, error) {
	row, err := s.accountForUser(ctx, userID, accountID)
	if err != nil {
		return AccountResponse{}, err
	}
	normalized, err := normalizeAccountInput(AccountInput{
		Name:              input.Name,
		Provider:          row.Provider,
		Endpoint:          input.Endpoint,
		Token:             input.Token,
		DefaultTargetID:   input.DefaultTargetID,
		DefaultTargetName: input.DefaultTargetName,
		Enabled:           input.Enabled,
		ClearCredentials:  input.ClearCredentials,
	})
	if err != nil {
		return AccountResponse{}, err
	}

	update := row.Update().
		SetName(normalized.Name).
		SetEndpoint(normalized.Endpoint).
		SetEnabled(normalized.Enabled).
		SetDefaultTargetID(normalized.DefaultTargetID).
		SetDefaultTargetName(normalized.DefaultTargetName)

	if normalized.ClearCredentials {
		update.ClearEncryptedCredentials().ClearCredentialAlg()
	} else if normalized.Token != nil && strings.TrimSpace(*normalized.Token) != "" {
		encrypted, err := s.encryptCredentials(Credentials{Token: strings.TrimSpace(*normalized.Token)})
		if err != nil {
			return AccountResponse{}, err
		}
		update.SetEncryptedCredentials(encrypted).SetCredentialAlg(credentialAlg)
	}

	row, err = update.Save(ctx)
	if err != nil {
		return AccountResponse{}, err
	}
	return accountResponse(row), nil
}

func (s *Service) DeleteAccount(ctx context.Context, userID, accountID int) error {
	row, err := s.accountForUser(ctx, userID, accountID)
	if err != nil {
		return err
	}

	tx, err := s.client.Tx(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	txClient := tx.Client()
	if _, err := txClient.NoteConnectionItemMap.Delete().
		Where(noteconnectionitemmap.AccountIDEQ(row.ID)).
		Exec(ctx); err != nil {
		return err
	}
	if _, err := txClient.NoteConnectionJob.Delete().
		Where(noteconnectionjob.AccountIDEQ(row.ID)).
		Exec(ctx); err != nil {
		return err
	}
	count, err := txClient.NoteConnectionAccount.Delete().
		Where(noteconnectionaccount.ID(accountID), noteconnectionaccount.UserIDEQ(userID)).
		Exec(ctx)
	if err != nil {
		return err
	}
	if count == 0 {
		return &ent.NotFoundError{}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *Service) TestUnsaved(ctx context.Context, input AccountInput) error {
	normalized, err := normalizeAccountInput(input)
	if err != nil {
		return err
	}
	if normalized.Token == nil || strings.TrimSpace(*normalized.Token) == "" {
		return ErrMissingCredential
	}
	provider, err := s.newProvider(normalized.Provider, normalized.Endpoint, Credentials{Token: strings.TrimSpace(*normalized.Token)})
	if err != nil {
		return err
	}
	return redactProviderError(provider.Test(ctx))
}

func (s *Service) TestAccount(ctx context.Context, userID, accountID int, tokenOverride *string) (AccountResponse, error) {
	row, err := s.accountForUser(ctx, userID, accountID)
	if err != nil {
		return AccountResponse{}, err
	}

	provider, err := s.providerForAccount(ctx, row, tokenOverride)
	now := time.Now()
	if err == nil {
		err = redactProviderError(provider.Test(ctx))
	}

	update := row.Update().SetLastTestAt(now)
	if err != nil {
		row, _ = update.SetLastTestStatus(StatusFailed).SetLastTestError(redactError(err)).Save(ctx)
		if row != nil {
			return accountResponse(row), err
		}
		return AccountResponse{}, err
	}
	row, err = update.SetLastTestStatus(StatusSuccess).ClearLastTestError().Save(ctx)
	if err != nil {
		return AccountResponse{}, err
	}
	return accountResponse(row), nil
}

func (s *Service) ListTargets(ctx context.Context, userID, accountID int) ([]Target, error) {
	row, err := s.accountForUser(ctx, userID, accountID)
	if err != nil {
		return nil, err
	}
	if err := ensureAccountEnabled(row); err != nil {
		return nil, err
	}
	provider, err := s.providerForAccount(ctx, row, nil)
	if err != nil {
		return nil, err
	}
	targets, err := provider.ListTargets(ctx)
	if err != nil {
		return nil, redactProviderError(err)
	}
	sort.SliceStable(targets, func(i, j int) bool {
		if targets[i].Kind == targets[j].Kind {
			return strings.ToLower(targets[i].Name) < strings.ToLower(targets[j].Name)
		}
		return targets[i].Kind < targets[j].Kind
	})
	return targets, nil
}

func (s *Service) ImportNotes(ctx context.Context, userID, accountID int, req ImportRequest) (*ImportResult, error) {
	row, err := s.accountForUser(ctx, userID, accountID)
	if err != nil {
		return nil, err
	}
	if err := ensureAccountEnabled(row); err != nil {
		return nil, err
	}
	provider, err := s.providerForAccount(ctx, row, nil)
	if err != nil {
		return nil, err
	}

	limit := clampLimit(req.Limit)
	job, err := s.client.NoteConnectionJob.Create().
		SetProvider(row.Provider).
		SetOperation(OperationImport).
		SetStatus(JobRunning).
		SetUserID(userID).
		SetAccountID(row.ID).
		SetTotalCount(0).
		SetStartedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return nil, err
	}

	remoteNotes, err := provider.ImportNotes(ctx, req.TargetID, limit)
	if err != nil {
		err = redactProviderError(err)
		_ = job.Update().
			SetStatus(JobFailed).
			SetFailedCount(1).
			SetMessage(redactError(err)).
			SetCompletedAt(time.Now()).
			Exec(ctx)
		return nil, err
	}

	result := &ImportResult{JobID: job.ID, TotalCount: len(remoteNotes)}
	for _, remote := range remoteNotes {
		if strings.TrimSpace(remote.ExternalID) == "" {
			result.FailedCount++
			continue
		}
		exists, err := s.client.NoteConnectionItemMap.Query().
			Where(
				noteconnectionitemmap.AccountIDEQ(row.ID),
				noteconnectionitemmap.ExternalIDEQ(remote.ExternalID),
			).
			Exist(ctx)
		if err != nil {
			result.FailedCount++
			continue
		}
		if exists {
			result.SkippedCount++
			continue
		}
		created, err := s.createImportedNote(ctx, userID, row, remote)
		if err != nil {
			result.FailedCount++
			continue
		}
		if err := s.saveItemMap(ctx, row, created.ID, remote.ExternalID, remote.TargetID, remote.Path, remote.URL, OperationImport); err != nil {
			result.FailedCount++
			continue
		}
		result.ImportedCount++
	}

	result.Status = jobStatus(result.ImportedCount, result.FailedCount)
	job, err = job.Update().
		SetStatus(result.Status).
		SetTotalCount(result.TotalCount).
		SetImportedCount(result.ImportedCount).
		SetSkippedCount(result.SkippedCount).
		SetFailedCount(result.FailedCount).
		SetCompletedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	result.JobID = job.ID
	return result, nil
}

func (s *Service) PushNote(ctx context.Context, userID, accountID int, noteUUID uuid.UUID, targetID string) (*PushResponse, error) {
	row, err := s.accountForUser(ctx, userID, accountID)
	if err != nil {
		return nil, err
	}
	if err := ensureAccountEnabled(row); err != nil {
		return nil, err
	}
	provider, err := s.providerForAccount(ctx, row, nil)
	if err != nil {
		return nil, err
	}

	noteRow, err := s.client.Note.Query().
		Where(note.IDEQ(noteUUID), note.HasUserWith(user.IDEQ(userID)), note.IsDeleted(false)).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	if noteRow.ProtectionMode != note.ProtectionModeNone {
		return nil, errors.New("protected notes cannot be pushed while locked")
	}

	existingID := ""
	existingMap, err := s.client.NoteConnectionItemMap.Query().
		Where(
			noteconnectionitemmap.AccountIDEQ(row.ID),
			noteconnectionitemmap.NoteIDEQ(noteUUID),
		).
		Only(ctx)
	if err == nil {
		existingID = existingMap.ExternalID
	} else if !ent.IsNotFound(err) {
		return nil, err
	}

	job, err := s.client.NoteConnectionJob.Create().
		SetProvider(row.Provider).
		SetOperation(OperationPush).
		SetStatus(JobRunning).
		SetUserID(userID).
		SetAccountID(row.ID).
		SetNoteID(noteUUID).
		SetTotalCount(1).
		SetStartedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return nil, err
	}

	result, err := provider.PushNote(ctx, PushInput{
		NoteID:             noteRow.ID,
		Title:              noteRow.Title,
		Content:            noteRow.Content,
		TargetID:           strings.TrimSpace(targetID),
		ExistingExternalID: existingID,
	})
	finished := time.Now()
	if err != nil {
		err = redactProviderError(err)
		_ = job.Update().
			SetStatus(JobFailed).
			SetFailedCount(1).
			SetMessage(redactError(err)).
			SetCompletedAt(finished).
			Exec(ctx)
		return nil, err
	}
	if err := s.saveItemMap(ctx, row, noteRow.ID, result.ExternalID, result.TargetID, result.Path, result.URL, OperationPush); err != nil {
		return nil, err
	}
	job, err = job.Update().
		SetStatus(JobCompleted).
		SetPushedCount(1).
		SetCompletedAt(finished).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return &PushResponse{
		JobID:      job.ID,
		Status:     job.Status,
		Result:     result,
		FinishedAt: finished,
	}, nil
}

func (s *Service) ListJobs(ctx context.Context, userID int) ([]*ent.NoteConnectionJob, error) {
	return s.client.NoteConnectionJob.Query().
		Where(noteconnectionjob.UserIDEQ(userID)).
		Order(ent.Desc(noteconnectionjob.FieldCreatedAt)).
		Limit(25).
		All(ctx)
}

func (s *Service) accountForUser(ctx context.Context, userID, accountID int) (*ent.NoteConnectionAccount, error) {
	return s.client.NoteConnectionAccount.Query().
		Where(noteconnectionaccount.ID(accountID), noteconnectionaccount.UserIDEQ(userID)).
		Only(ctx)
}

func ensureAccountEnabled(row *ent.NoteConnectionAccount) error {
	if row != nil && !row.Enabled {
		return ErrAccountDisabled
	}
	return nil
}

func (s *Service) providerForAccount(ctx context.Context, row *ent.NoteConnectionAccount, tokenOverride *string) (Provider, error) {
	credentials := Credentials{}
	if tokenOverride != nil && strings.TrimSpace(*tokenOverride) != "" {
		credentials.Token = strings.TrimSpace(*tokenOverride)
	} else {
		var err error
		credentials, err = s.decryptCredentials(row)
		if err != nil {
			return nil, err
		}
	}
	return s.newProvider(row.Provider, row.Endpoint, credentials)
}

func (s *Service) newProvider(provider, endpoint string, credentials Credentials) (Provider, error) {
	if strings.TrimSpace(credentials.Token) == "" {
		return nil, ErrMissingCredential
	}
	endpoint, err := normalizeEndpoint(provider, endpoint)
	if err != nil {
		return nil, err
	}
	switch provider {
	case ProviderSiYuan:
		return newSiYuanProvider(endpoint, credentials.Token, s.http)
	case ProviderNotion:
		return newNotionProvider(credentials.Token, s.http), nil
	case ProviderJoplin:
		return newJoplinProvider(endpoint, credentials.Token, s.http), nil
	default:
		return nil, ErrUnsupportedProvider
	}
}

func (s *Service) encryptCredentials(credentials Credentials) (string, error) {
	raw, err := json.Marshal(credentials)
	if err != nil {
		return "", err
	}
	return s.box.Seal(raw)
}

func (s *Service) decryptCredentials(row *ent.NoteConnectionAccount) (Credentials, error) {
	if row.EncryptedCredentials == "" {
		return Credentials{}, ErrMissingCredential
	}
	raw, err := s.box.Open(row.EncryptedCredentials)
	if err != nil {
		return Credentials{}, err
	}
	var credentials Credentials
	if err := json.Unmarshal(raw, &credentials); err != nil {
		return Credentials{}, err
	}
	if strings.TrimSpace(credentials.Token) == "" {
		return Credentials{}, ErrMissingCredential
	}
	return credentials, nil
}

func (s *Service) createImportedNote(ctx context.Context, userID int, account *ent.NoteConnectionAccount, remote RemoteNote) (*ent.Note, error) {
	title := titleOrUntitled(remote.Title)
	create := s.client.Note.Create().
		SetTitle(title).
		SetContent(remote.Content).
		SetUserID(userID)
	if remote.CreatedAt != nil {
		create.SetCreatedAt(*remote.CreatedAt)
	}
	if remote.UpdatedAt != nil {
		create.SetUpdatedAt(*remote.UpdatedAt)
	}
	if remote.TargetName != "" {
		folderRow, err := s.findOrCreateFolder(ctx, userID, remote.TargetName)
		if err != nil {
			return nil, err
		}
		create.SetFolderID(folderRow.ID)
	}
	return create.Save(ctx)
}

func (s *Service) findOrCreateFolder(ctx context.Context, userID int, name string) (*ent.Folder, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrMissingTarget
	}
	existing, err := s.client.Folder.Query().
		Where(folder.NameEQ(name), folder.HasUserWith(user.IDEQ(userID)), folder.Not(folder.HasParent())).
		Order(ent.Asc(folder.FieldCreatedAt)).
		First(ctx)
	if err == nil {
		return existing, nil
	}
	if !ent.IsNotFound(err) {
		return nil, err
	}
	return s.client.Folder.Create().
		SetName(name).
		SetUserID(userID).
		Save(ctx)
}

func (s *Service) saveItemMap(ctx context.Context, account *ent.NoteConnectionAccount, noteID uuid.UUID, externalID, targetID, path, externalURL, direction string) error {
	now := time.Now()
	existing, err := s.client.NoteConnectionItemMap.Query().
		Where(noteconnectionitemmap.AccountIDEQ(account.ID), noteconnectionitemmap.NoteIDEQ(noteID)).
		Only(ctx)
	if err == nil {
		update := existing.Update().
			SetExternalID(externalID).
			SetExternalTargetID(targetID).
			SetExternalPath(path).
			SetExternalURL(externalURL).
			SetLastSyncDirection(direction)
		if direction == OperationImport {
			update.SetLastImportedAt(now)
		} else {
			update.SetLastPushedAt(now)
		}
		return update.Exec(ctx)
	}
	if !ent.IsNotFound(err) {
		return err
	}

	create := s.client.NoteConnectionItemMap.Create().
		SetProvider(account.Provider).
		SetAccountID(account.ID).
		SetNoteID(noteID).
		SetExternalID(externalID).
		SetExternalTargetID(targetID).
		SetExternalPath(path).
		SetExternalURL(externalURL).
		SetLastSyncDirection(direction)
	if direction == OperationImport {
		create.SetLastImportedAt(now)
	} else {
		create.SetLastPushedAt(now)
	}
	return create.Exec(ctx)
}

func accountResponse(row *ent.NoteConnectionAccount) AccountResponse {
	return AccountResponse{
		ID:                row.ID,
		Name:              row.Name,
		Provider:          row.Provider,
		Endpoint:          row.Endpoint,
		Enabled:           row.Enabled,
		AuthType:          row.AuthType,
		HasCredentials:    row.EncryptedCredentials != "",
		DefaultTargetID:   row.DefaultTargetID,
		DefaultTargetName: row.DefaultTargetName,
		LastTestStatus:    row.LastTestStatus,
		LastTestError:     row.LastTestError,
		LastTestAt:        timePtr(row.LastTestAt),
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func normalizeAccountInput(input AccountInput) (AccountInput, error) {
	provider := strings.ToLower(strings.TrimSpace(input.Provider))
	switch provider {
	case ProviderSiYuan, ProviderNotion, ProviderJoplin:
	default:
		return input, ErrUnsupportedProvider
	}
	endpoint, err := normalizeEndpoint(provider, input.Endpoint)
	if err != nil {
		return input, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = defaultAccountName(provider)
	}
	if len([]rune(name)) > 80 {
		name = string([]rune(name)[:80])
	}
	input.Name = name
	input.Provider = provider
	input.Endpoint = endpoint
	input.DefaultTargetID = strings.TrimSpace(input.DefaultTargetID)
	input.DefaultTargetName = strings.TrimSpace(input.DefaultTargetName)
	return input, nil
}

func normalizeEndpoint(provider, endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	switch provider {
	case ProviderNotion:
		return "", nil
	case ProviderSiYuan:
		if endpoint == "" {
			endpoint = "http://127.0.0.1:6806"
		}
	case ProviderJoplin:
		if endpoint == "" {
			endpoint = "http://127.0.0.1:41184"
		}
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("invalid endpoint: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("endpoint must use http or https")
	}
	if parsed.Host == "" {
		return "", errors.New("endpoint host is required")
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func defaultAccountName(provider string) string {
	switch provider {
	case ProviderSiYuan:
		return "SiYuan"
	case ProviderNotion:
		return "Notion"
	case ProviderJoplin:
		return "Joplin"
	default:
		return "Note Account"
	}
}

func titleOrUntitled(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "Untitled"
	}
	if len([]rune(title)) > 240 {
		return string([]rune(title)[:240])
	}
	return title
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

func jobStatus(success, failed int) string {
	if failed == 0 {
		return JobCompleted
	}
	if success > 0 {
		return JobCompletedWithErrors
	}
	return JobFailed
}

func redactError(err error) string {
	if err == nil {
		return ""
	}
	message := sensitiveQueryValuePattern.ReplaceAllString(err.Error(), "${1}REDACTED")
	if len(message) > 500 {
		message = message[:500]
	}
	return message
}

func redactProviderError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrUnsupportedProvider) ||
		errors.Is(err, ErrMissingCredential) ||
		errors.Is(err, ErrMissingTarget) ||
		errors.Is(err, ErrAccountDisabled) {
		return err
	}
	return errors.New(redactError(err))
}
