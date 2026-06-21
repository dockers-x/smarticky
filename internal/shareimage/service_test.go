package shareimage

import (
	"context"
	"image/png"
	"os"
	"testing"

	"smarticky/ent/enttest"

	_ "github.com/lib-x/entsqlite"
)

func TestGenerateCreatesOwnedPNG(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestGenerateCreatesOwnedPNG?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	owner := client.User.Create().SetUsername("owner").SetPasswordHash("hash").SaveX(ctx)
	other := client.User.Create().SetUsername("other").SetPasswordHash("hash").SaveX(ctx)

	service := NewService(client, t.TempDir())
	generated, err := service.Generate(ctx, owner.ID, GenerateInput{
		Title:   "长文标题",
		Content: "第一段内容\n第二段内容",
		Theme:   "classic",
		Ratio:   "story",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if generated.Size == 0 {
		t.Fatal("expected generated image to have non-zero size")
	}

	imageFile, err := service.GetOwnedImage(ctx, owner.ID, generated.ID)
	if err != nil {
		t.Fatalf("GetOwnedImage returned error: %v", err)
	}
	raw, err := os.Open(imageFile.Path)
	if err != nil {
		t.Fatalf("generated file not readable: %v", err)
	}
	defer raw.Close()
	if _, err := png.DecodeConfig(raw); err != nil {
		t.Fatalf("generated file is not a valid PNG: %v", err)
	}

	if _, err := service.GetOwnedImage(ctx, other.ID, generated.ID); err == nil {
		t.Fatal("expected other user to be denied image access")
	}
}
