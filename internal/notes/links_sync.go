package notes

import (
	"context"
	"fmt"

	"smarticky/ent"
	"smarticky/ent/note"
	"smarticky/ent/notelink"
	"smarticky/ent/user"

	"github.com/google/uuid"
)

func (s *Service) SyncNoteLinks(ctx context.Context, userID int, sourceNoteID uuid.UUID) error {
	source, err := s.client.Note.Query().
		Where(note.IDEQ(sourceNoteID), note.HasUserWith(user.IDEQ(userID))).
		Only(ctx)
	if err != nil {
		return err
	}

	var resolvedLinks []resolvedLink
	if source.ProtectionMode != note.ProtectionModeEncrypted {
		resolvedLinks, err = s.resolveParsedLinks(ctx, userID, ParseWikiLinks(source.Content))
		if err != nil {
			return err
		}
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

	if _, err := tx.NoteLink.Delete().
		Where(notelink.UserIDEQ(userID), notelink.SourceNoteIDEQ(sourceNoteID)).
		Exec(ctx); err != nil {
		return err
	}

	if source.ProtectionMode == note.ProtectionModeEncrypted || len(resolvedLinks) == 0 {
		if err := tx.Commit(); err != nil {
			return err
		}
		committed = true
		return nil
	}

	builders := make([]*ent.NoteLinkCreate, 0, len(resolvedLinks))
	for _, link := range resolvedLinks {
		builder := tx.NoteLink.Create().
			SetUserID(userID).
			SetSourceNoteID(sourceNoteID).
			SetTargetRef(link.TargetRef).
			SetTargetRefNorm(link.TargetRefNorm).
			SetTargetKey(link.TargetKey).
			SetDisplayText(link.DisplayText).
			SetLinkType(notelink.LinkTypeWiki).
			SetOccurrenceCount(link.OccurrenceCount)
		if link.TargetNoteID != nil {
			builder.SetTargetNoteID(*link.TargetNoteID)
		}
		builders = append(builders, builder)
	}
	if _, err := tx.NoteLink.CreateBulk(builders...).Save(ctx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *Service) SyncUserLinks(ctx context.Context, userID int) error {
	ids, err := s.client.Note.Query().
		Where(note.HasUserWith(user.IDEQ(userID))).
		IDs(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.SyncNoteLinks(ctx, userID, id); err != nil {
			return err
		}
	}
	return nil
}

type resolvedLink struct {
	ParsedLink
	TargetNoteID *uuid.UUID
	TargetKey    string
}

func (s *Service) resolveParsedLinks(ctx context.Context, userID int, parsed []ParsedLink) ([]resolvedLink, error) {
	resolved := make([]resolvedLink, 0, len(parsed))
	for _, link := range parsed {
		targetID, err := s.resolveLinkTarget(ctx, userID, link.TargetRef, link.TargetRefNorm)
		if err != nil {
			return nil, err
		}

		targetKey := "title:" + link.TargetRefNorm
		if targetID != nil {
			targetKey = "note:" + targetID.String()
		}
		resolved = append(resolved, resolvedLink{
			ParsedLink:   link,
			TargetNoteID: targetID,
			TargetKey:    targetKey,
		})
	}
	return resolved, nil
}

func (s *Service) resolveLinkTarget(ctx context.Context, userID int, targetRef string, targetRefNorm string) (*uuid.UUID, error) {
	exact, err := s.client.Note.Query().
		Where(note.TitleEQ(targetRef), note.HasUserWith(user.IDEQ(userID))).
		All(ctx)
	if err != nil {
		return nil, err
	}
	if len(exact) == 1 {
		id := exact[0].ID
		return &id, nil
	}
	if len(exact) > 1 {
		return nil, nil
	}

	rows, err := s.client.Note.Query().
		Where(note.HasUserWith(user.IDEQ(userID))).
		All(ctx)
	if err != nil {
		return nil, err
	}
	var match *uuid.UUID
	for _, row := range rows {
		if normalizeLinkRef(row.Title) != targetRefNorm {
			continue
		}
		if match != nil {
			return nil, nil
		}
		id := row.ID
		match = &id
	}
	return match, nil
}

func (s *Service) DeleteLinksForNotes(ctx context.Context, userID int, noteIDs ...uuid.UUID) error {
	if len(noteIDs) == 0 {
		return nil
	}
	_, err := s.client.NoteLink.Delete().
		Where(
			notelink.UserIDEQ(userID),
			notelink.Or(
				notelink.SourceNoteIDIn(noteIDs...),
				notelink.TargetNoteIDIn(noteIDs...),
			),
		).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete note links: %w", err)
	}
	return nil
}
