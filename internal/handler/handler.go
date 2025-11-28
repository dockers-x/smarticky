package handler

import (
	"smarticky/ent"
	"smarticky/internal/storage"
)

type Handler struct {
	client *ent.Client
	fs     *storage.FileSystem
}

func NewHandler(client *ent.Client, fs *storage.FileSystem) *Handler {
	return &Handler{
		client: client,
		fs:     fs,
	}
}
