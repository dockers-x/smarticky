package notetools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInstalledToolsAndRuntimeIsolation(t *testing.T) {
	dir := t.TempDir()
	for name, source := range map[string]string{
		"counter.js": "let count = 0; export function run(note) { return note.title + (++count) }",
		"invalid.js": "export definitely not a module",
		"long.js":    "export function run() { return '字'.repeat(24000) }",
		"slow.js":    "export function run() { for (;;) {} }",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	tasks, err := r.Run(ctx, "open-tasks", "", "- [ ] write\n- [x] done\n~~~\n- [ ] code\n~~~\n  * [ ] edit")
	if err != nil || tasks != "- [ ] write\n* [ ] edit" {
		t.Fatalf("open tasks = %q, %v", tasks, err)
	}
	for range 2 {
		if out, err := r.Run(ctx, "counter", "count ", ""); out != "count 1" || err != nil {
			t.Fatalf("fresh module output = %q, %v", out, err)
		}
	}
	if _, err := r.Run(ctx, "invalid", "", ""); !errors.Is(err, ErrUnknown) {
		t.Fatalf("invalid file was registered: %v", err)
	}
	if _, err := r.Run(ctx, "long", "", ""); err == nil {
		t.Fatal("accepted excessive UTF-8 output")
	}
	if _, err := r.Run(ctx, "counter", "", strings.Repeat("a", (4<<20)+1)); err == nil {
		t.Fatal("accepted excessive input")
	}
	timeout, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	if _, err := r.Run(timeout, "slow", "", ""); err == nil {
		t.Fatal("expected cancellation")
	}
	if out, err := r.Run(ctx, "counter", "count ", ""); out != "count 1" || err != nil {
		t.Fatalf("after timeout = %q, %v", out, err)
	}
}
