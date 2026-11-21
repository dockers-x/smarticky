// Package notetools runs operator-installed read-only tools against one note.
package notetools

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MoeForge/moejs"
	"go.uber.org/zap"
)

//go:embed open-tasks.js
var openTasks string

var ErrUnknown = errors.New("unknown note tool")

const maxOutput = 64 << 10

type Tool struct {
	ID     string `json:"id"`
	module *moejs.Module
	hook   moejs.Hook
}

type Registry struct {
	tools []Tool
	slots chan struct{}
}

func Open(dir string) (*Registry, error) {
	r := &Registry{slots: make(chan struct{}, 4)}
	if err := r.add("open-tasks", openTasks); err != nil {
		return nil, err
	}
	files, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return r, nil
	}
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		name := file.Name()
		id, ok := strings.CutSuffix(name, ".js")
		if !ok || !file.Type().IsRegular() || strings.HasPrefix(name, ".") || id == "open-tasks" {
			continue
		}
		if err := r.loadFile(id, filepath.Join(dir, name)); err != nil {
			zap.L().Warn("Skipping note tool", zap.String("file", name), zap.Error(err))
		}
	}
	return r, nil
}

func (r *Registry) loadFile(id, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	const limit = 128 << 10
	src, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return err
	}
	if len(src) > limit {
		return errors.New("tool source exceeds 128 KiB")
	}
	return r.add(id, string(src))
}

func (r *Registry) add(id, source string) error {
	mod, err := moejs.Compile(id+".js", source)
	if err != nil {
		return err
	}
	hook, err := mod.Hook("run")
	if err != nil {
		return err
	}
	r.tools = append(r.tools, Tool{ID: id, module: mod, hook: hook})
	return nil
}

func (r *Registry) List() []Tool { return r.tools }

func (r *Registry) Run(ctx context.Context, id, title, content string) (string, error) {
	for _, tool := range r.tools {
		if tool.ID != id {
			continue
		}
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		select {
		case r.slots <- struct{}{}:
			defer func() { <-r.slots }()
		case <-ctx.Done():
			return "", ctx.Err()
		}
		if len(title)+len(content) > 4<<20 {
			return "", errors.New("note exceeds the 4 MiB input limit")
		}
		rt := moejs.NewRuntime(moejs.Options{DisableDynamicCode: true, MemoryLimit: 16 << 20})
		stop := context.AfterFunc(ctx, func() { rt.Interrupt(ctx.Err()) })
		defer stop()
		if err := rt.Load(tool.module); err != nil {
			return "", err
		}
		input, err := rt.FromGo(map[string]string{"title": title, "content": content})
		if err != nil {
			return "", err
		}
		result, err := rt.Call(tool.hook, input)
		if err != nil {
			return "", err
		}
		if !result.IsString() {
			return "", errors.New("tool must return a string")
		}
		if result.AsString().Len() <= maxOutput {
			output := result.AsString().GoString()
			if len(output) <= maxOutput {
				return output, nil
			}
		}
		return "", fmt.Errorf("tool output exceeds %d bytes", maxOutput)
	}
	return "", ErrUnknown
}
