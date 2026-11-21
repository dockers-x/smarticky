package notes

import (
	"context"
	"slices"

	"smarticky/ent"
	"smarticky/ent/note"
	"smarticky/ent/notelink"

	"github.com/google/uuid"
)

// SyncChangedLinks updates outgoing links and references affected by a new,
// renamed or removed title in the same transaction as the note change.
// For a deletion changed is nil and SQLite has already detached incoming links.
func SyncChangedLinks(ctx context.Context, tx *ent.Tx, userID int, changed *ent.Note, titles ...string) error {
	var sources []*ent.Note
	if changed != nil {
		sources = append(sources, changed)
	}
	if len(titles) > 0 {
		refs := make([]string, 0, len(titles))
		for _, title := range titles {
			refs = append(refs, normalizeLinkRef(title))
		}
		q := tx.Note.Query().Where(note.UserIDEQ(userID),
			note.HasOutgoingLinksWith(notelink.UserIDEQ(userID), notelink.TargetRefNormIn(refs...)))
		if changed != nil {
			q.Where(note.IDNEQ(changed.ID))
		}
		related, err := q.Select(note.FieldID, note.FieldContent, note.FieldProtectionMode).All(ctx)
		if err != nil {
			return err
		}
		sources = append(sources, related...)
	}
	return replaceSourceLinks(ctx, tx, userID, sources)
}

func (s *Service) SyncNoteLinks(ctx context.Context, userID int, sourceID uuid.UUID) error {
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	row, err := tx.Note.Query().Where(note.UserIDEQ(userID), note.IDEQ(sourceID)).Only(ctx)
	if err != nil {
		return err
	}
	if err := SyncChangedLinks(ctx, tx, userID, row); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) SyncUserLinks(ctx context.Context, userID int) error {
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.Note.Query().Where(note.UserIDEQ(userID)).
		Select(note.FieldID, note.FieldContent, note.FieldProtectionMode).All(ctx)
	if err != nil {
		return err
	}
	if err := replaceSourceLinks(ctx, tx, userID, rows); err != nil {
		return err
	}
	return tx.Commit()
}

type linkTitles struct {
	exact, normalized map[string]uuid.UUID
}

func indexLinkTitle(index map[string]uuid.UUID, title string, id uuid.UUID) {
	if _, duplicate := index[title]; duplicate {
		id = uuid.Nil
	}
	index[title] = id
}

func loadLinkTitles(ctx context.Context, tx *ent.Tx, userID int) (*linkTitles, error) {
	rows, err := tx.Note.Query().Where(note.UserIDEQ(userID)).Select(note.FieldID, note.FieldTitle).All(ctx)
	if err != nil {
		return nil, err
	}
	titles := &linkTitles{make(map[string]uuid.UUID, len(rows)), make(map[string]uuid.UUID, len(rows))}
	for _, row := range rows {
		indexLinkTitle(titles.exact, row.Title, row.ID)
		indexLinkTitle(titles.normalized, normalizeLinkRef(row.Title), row.ID)
	}
	return titles, nil
}

func replaceSourceLinks(ctx context.Context, tx *ent.Tx, userID int, sources []*ent.Note) error {
	var titles *linkTitles
	for _, source := range sources {
		if _, err := tx.NoteLink.Delete().Where(notelink.UserIDEQ(userID), notelink.SourceNoteIDEQ(source.ID)).Exec(ctx); err != nil {
			return err
		}
		if source.ProtectionMode == note.ProtectionModeEncrypted {
			continue
		}
		links := ParseWikiLinks(source.Content)
		if len(links) == 0 {
			continue
		}
		if titles == nil {
			var err error
			titles, err = loadLinkTitles(ctx, tx, userID)
			if err != nil {
				return err
			}
		}
		for batch := range slices.Chunk(links, 128) {
			builders := make([]*ent.NoteLinkCreate, 0, len(batch))
			for _, link := range batch {
				target, exact := titles.exact[link.TargetRef]
				if !exact {
					target = titles.normalized[link.TargetRefNorm]
				}
				builder := tx.NoteLink.Create().SetUserID(userID).SetSourceNoteID(source.ID).
					SetTargetRef(link.TargetRef).SetTargetRefNorm(link.TargetRefNorm).
					SetDisplayText(link.DisplayText).SetOccurrenceCount(link.OccurrenceCount).
					SetTargetKey("title:" + link.TargetRefNorm)
				if target != uuid.Nil {
					builder.SetTargetNoteID(target).SetTargetKey("note:" + target.String())
				}
				builders = append(builders, builder)
			}
			if err := tx.NoteLink.CreateBulk(builders...).Exec(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}
