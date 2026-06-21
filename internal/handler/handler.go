package handler

import (
	"smarticky/ent"
	importsvc "smarticky/internal/importer"
	"smarticky/internal/notes"
	"smarticky/internal/shareimage"
	"smarticky/internal/storage"
)

type Handler struct {
	client      *ent.Client
	fs          *storage.FileSystem
	importer    *importsvc.Service
	notes       *notes.Service
	shareImages *shareimage.Service
}

func NewHandler(client *ent.Client, fs *storage.FileSystem) *Handler {
	if fs == nil {
		fs = storage.NewMemoryFileSystem()
	}

	return &Handler{
		client:      client,
		fs:          fs,
		importer:    importsvc.NewService(client, fs),
		notes:       notes.NewService(client),
		shareImages: shareimage.NewService(client, fs.GetDataDir()),
	}
}

func (h *Handler) NotesService() *notes.Service {
	return h.notes
}

func (h *Handler) ShareImageService() *shareimage.Service {
	return h.shareImages
}
