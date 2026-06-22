package handler

import (
	"smarticky/ent"
	importsvc "smarticky/internal/importer"
	"smarticky/internal/notes"
	searchsvc "smarticky/internal/search"
	"smarticky/internal/shareimage"
	"smarticky/internal/storage"
)

type Handler struct {
	client      *ent.Client
	fs          *storage.FileSystem
	importer    *importsvc.Service
	notes       *notes.Service
	search      *searchsvc.Service
	shareImages *shareimage.Service
}

func NewHandler(client *ent.Client, fs *storage.FileSystem) *Handler {
	return NewHandlerWithSearch(client, fs, nil)
}

func NewHandlerWithSearch(client *ent.Client, fs *storage.FileSystem, searchService *searchsvc.Service) *Handler {
	if fs == nil {
		fs = storage.NewMemoryFileSystem()
	}

	return &Handler{
		client:      client,
		fs:          fs,
		importer:    importsvc.NewService(client, fs),
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
