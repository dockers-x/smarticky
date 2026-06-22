# Smarticky Backup Targets Design

## Goal

Replace the single WebDAV/S3 backup configuration with a multi-target backup system.
Each backup target is an independent remote destination that can be enabled, scheduled,
run manually, listed, verified, and used for restore.

The feature must preserve existing user data. Schema migration may add new tables and
columns, but must not rebuild tables manually, wipe backup settings, or delete note data.
Remote restore remains an explicit destructive operation and requires a typed confirmation.

## Current State

The current backend stores backup settings in one `BackupConfig` row:

- WebDAV credentials and URL
- S3 endpoint, bucket, and credentials
- global `auto_backup_enabled`
- global `backup_schedule`
- global retention settings
- `folder_max_depth`, which is unrelated to backups but still depends on `BackupConfig`

Automatic backup currently checks once per day and writes to WebDAV first. If WebDAV
succeeds, S3 is skipped. This does not satisfy the requirement to back up to multiple
remote objects.

## Data Model

Add a new `BackupTarget` Ent schema.

Fields:

- `id`: int primary key
- `name`: display name, required
- `type`: enum string, `webdav` or `s3`
- `enabled`: bool, default `true`
- `schedule`: enum string, `manual`, `daily`, `weekly`, `monthly`, default `manual`
- `retention_days`: int, default `30`, where `0` means no age limit
- `max_count`: int, default `10`, where `0` means no count limit
- `last_backup_at`: optional time
- `last_backup_status`: enum string, `never`, `success`, `failed`, default `never`
- `last_backup_error`: optional string
- `last_test_at`: optional time
- `last_test_status`: enum string, `never`, `success`, `failed`, default `never`
- `last_test_error`: optional string
- `created_at`, `updated_at`
- WebDAV fields: `webdav_url`, `webdav_user`, `webdav_password`
- S3 fields: `s3_endpoint`, `s3_region`, `s3_bucket`, `s3_access_key`, `s3_secret_key`

Secret fields must be marked sensitive in Ent and must not be logged. API responses must
not echo secret values back to the frontend. Existing `BackupConfig` remains because
`folder_max_depth` uses it. Old backup fields on `BackupConfig` are not used by the new UI
or API.

## Existing Config Migration

On first access to backup targets, if no `BackupTarget` rows exist:

- If `BackupConfig.webdav_url` is set, create a WebDAV target from the old fields.
- If `BackupConfig.s3_endpoint` and `BackupConfig.s3_bucket` are set, create an S3 target.
- Copy global schedule and retention values to created targets.
- If old `auto_backup_enabled` is false, created targets use `schedule = manual`.
- Do not clear old fields during this migration.

This migration is additive and idempotent. It must be safe to run multiple times.

## API Contract

Old backup APIs are removed from the active frontend and do not need compatibility:

- no `/backup/webdav`
- no `/backup/s3`
- no `/restore/webdav`
- no `/restore/s3`
- no `/backup/list/webdav`
- no `/backup/list/s3`
- no `/backup/verify/webdav`
- no `/backup/verify/s3`

New protected APIs:

- `GET /api/backup/targets`
  - Returns all targets with secret fields redacted.
- `POST /api/backup/targets`
  - Creates one target.
- `POST /api/backup/targets/test`
  - Tests an unsaved target payload from the create/edit form.
- `PUT /api/backup/targets/:id`
  - Updates one target.
- `DELETE /api/backup/targets/:id`
  - Deletes one target config only. It does not delete remote backup files.
- `POST /api/backup/targets/:id/test`
  - Tests one saved target using stored configuration.
- `POST /api/backup/targets/:id/run`
  - Runs a manual backup for that target.
- `GET /api/backup/targets/:id/files`
  - Lists remote backup files for that target.
- `POST /api/backup/targets/:id/verify`
  - Verifies one remote file without restoring.
- `POST /api/backup/targets/:id/restore`
  - Restores one remote file after fixed phrase confirmation.

Restore request:

```json
{
  "filename": "smarticky_backup_20260623_020000.tar.gz",
  "confirmation": "RESTORE"
}
```

Restore rejects any confirmation value other than `RESTORE`.

Connection test response:

```json
{
  "ok": true,
  "message": "connection test successful",
  "checked_at": "2026-06-23T02:00:00Z"
}
```

Failed tests return `ok: false` with a generic `message` that explains the failed
operation without exposing credentials.

Validation rules:

- Target `type` must be `webdav` or `s3`.
- Target `schedule` must be `manual`, `daily`, `weekly`, or `monthly`.
- `retention_days` and `max_count` must be non-negative.
- WebDAV target requires `webdav_url`.
- S3 target requires `s3_endpoint`, `s3_bucket`, `s3_access_key`, and `s3_secret_key`.
- Filename must match Smarticky backup naming patterns and must not contain path traversal.

## Backup Execution

Create a small internal target abstraction:

- `backupTargetClient.List(ctx) ([]BackupFileInfo, error)`
- `backupTargetClient.Upload(ctx, filename string, data []byte) error`
- `backupTargetClient.Download(ctx, filename string) ([]byte, error)`
- `backupTargetClient.Delete(ctx, filename string) error`
- `backupTargetClient.Test(ctx) error`

Implement WebDAV and S3 clients behind that interface. The existing archive creation,
WAL checkpoint, tar path safety, verification, extraction, and cleanup behavior should be
reused.

Connection testing:

- New target forms call `POST /api/backup/targets/test` with the current form payload,
  including secrets. Nothing is persisted by this endpoint.
- Existing target rows call `POST /api/backup/targets/:id/test` and use stored secrets.
- Saved target tests update `last_test_at`, `last_test_status`, and `last_test_error`.
- Unsaved target tests only update local UI state. After a new target is saved, the UI can
  run the saved-target test endpoint to persist the last test state.
- WebDAV test runs a minimal probe: list the target directory, write a small temporary
  probe file, read it back, then delete it.
- S3 test runs a minimal probe: list the bucket/prefix, put a small temporary probe object,
  read or head it, then delete it.
- Any probe cleanup failure makes the test fail because stale probe files indicate the
  target lacks permissions needed for retention cleanup.
- Probe filenames must use a reserved prefix such as `.smarticky_connection_test_` and
  must never match normal backup filename patterns.
- Test failures return a generic, user-actionable message and must not include credentials.

Manual backup:

1. Load target.
2. Validate target configuration.
3. Checkpoint WAL.
4. Create one backup archive.
5. Upload to selected target.
6. Update target status and retention cleanup.

Scheduled backup:

1. Scheduler runs once daily at the existing fixed time.
2. Load enabled targets whose schedule is due.
3. If at least one target is due, checkpoint WAL once and create one archive once.
4. Upload the same archive to each due target.
5. Target failures do not block other targets.
6. Update each target's last status independently.

Due rules:

- `manual`: never due
- `daily`: due every scheduler run
- `weekly`: due on Sunday
- `monthly`: due on day 1 of the month

## Restore Safety

Remote restore is destructive because it overwrites the current database and uploads.
The UI and backend must make this explicit.

Restore flow:

1. User opens a target's file list.
2. User chooses a file and clicks restore.
3. UI verifies the selected backup file first.
4. UI shows a high-risk confirmation dialog with:
   - target name and type
   - filename
   - file size and timestamp
   - verification result
   - warning that current data will be overwritten
   - warning that app restart is required
5. User must type `RESTORE`.
6. Backend re-checks confirmation and filename.
7. Backend downloads the file, verifies archive safety, checkpoints WAL, creates a local
   `smarticky_pre_restore_backup_*.tar.gz`, extracts the backup, removes SQLite sidecars,
   and returns `restart_required: true`.

If the pre-restore backup fails, restore must abort. If verification fails, UI should not
offer the final restore button.

## Frontend UX

Replace the current three-tab backup panel with target management:

- Main view: list of backup targets with name, type, enabled state, schedule, last status,
  and last backup time.
- Actions per target: edit, run now, files, delete.
- Add target flow: choose WebDAV or S3, then show only relevant fields.
- Add and edit forms include a required visible "Test connection" button.
- The test button validates the current form values before saving and shows inline success
  or failure state near the form actions.
- Saving is allowed even if the connection has not been tested, but enabling scheduled
  backup on an untested target shows a warning state in the form. Manual and scheduled
  backup still validate the target again before uploading.
- Schedule selector: manual, daily, weekly, monthly.
- Retention controls live on each target.
- File list view is scoped to one target.
- Restore confirmation uses a typed `RESTORE` field and disables submit until it matches.

The UI should keep the existing settings visual language: compact operational layout,
clear form labels, 44px touch targets, and inline error states.

## Security And Reliability

- Never return stored passwords/secrets in API responses.
- Never log target credentials.
- Validate all target input at API boundary.
- Use path-safe filename checks before remote download and restore extraction.
- Keep restore protected by existing JWT auth; restore is available only to authenticated
  users through the existing protected route group.
- Remote target deletion deletes local target config only, not remote backup files.
- Scheduled backup should log failures without crashing the server.
- Do not close over stale `BackupConfig` for scheduling; load targets each scheduled run.

## Testing Plan

Backend:

- Ent schema migration compiles.
- Backup target API validates invalid type, invalid schedule, negative retention, and
  missing required backend fields.
- Existing old config is migrated into targets when no targets exist.
- Scheduled due calculation covers manual, daily, weekly, and monthly.
- Manual backup uses one selected target and updates status.
- Restore rejects missing or wrong `RESTORE` confirmation.
- Restore rejects path traversal filenames and unsafe archives.
- Cleanup keeps newest files by target retention settings.

Frontend:

- Svelte type check covers new API types and target UI state.
- Target forms render backend-specific fields only.
- Restore submit remains disabled until typed confirmation is exactly `RESTORE`.
- Mobile settings layout remains usable.

Release verification:

- `go test ./... -count=1`
- `cd web/app && mise exec -- npm run check`
- `cd web/app && mise exec -- npm test`
- `cd web/app && mise exec -- npm run build`
