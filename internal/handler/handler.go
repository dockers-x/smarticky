package handler

import (
	"smarticky/ent"
	importsvc "smarticky/internal/importer"
	"smarticky/internal/storage"
)

type Handler struct {
	client   *ent.Client
	fs       *storage.FileSystem
	importer *importsvc.Service
}

func NewHandler(client *ent.Client, fs *storage.FileSystem) *Handler {
	return &Handler{
		client:   client,
		fs:       fs,
		importer: importsvc.NewService(client, fs),
	}
}
