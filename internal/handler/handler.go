package handler

import (
	"smarticky/ent"
	connectsvc "smarticky/internal/connections"
	importsvc "smarticky/internal/importer"
	"smarticky/internal/notes"
	searchsvc "smarticky/internal/search"
	"smarticky/internal/secrets"
	"smarticky/internal/shareimage"
	"smarticky/internal/storage"

	"github.com/lib-x/timewheel/scheduler"
)

type Handler struct {
	client          *ent.Client
	fs              *storage.FileSystem
	importer        *importsvc.Service
	connections     *connectsvc.Service
	notes           *notes.Service
	search          *searchsvc.Service
	shareImages     *shareimage.Service
	backupScheduler *scheduler.Scheduler[int, backupScheduleData]
}

func NewHandler(client *ent.Client, fs *storage.FileSystem) *Handler {
	return NewHandlerWithSearch(client, fs, nil)
}

func NewHandlerWithSearch(client *ent.Client, fs *storage.FileSystem, searchService *searchsvc.Service) *Handler {
	if fs == nil {
		fs = storage.NewMemoryFileSystem()
	}

	box, _ := secrets.OpenBox(fs)

	return &Handler{
		client:      client,
		fs:          fs,
		importer:    importsvc.NewService(client, fs),
		connections: connectsvc.NewService(client, box),
		notes:       notes.NewService(client, searchService),
		search:      searchService,
		shareImages: shareimage.NewService(client, fs.GetDataDir()),
	}
}

func (h *Handler) NotesService() *notes.Service {
	return h.notes
}

func (h *Handler) ShareImageService() *shareimage.Service {
	return h.shareImages
}
