package web

import "testing"

func TestEmbeddedStaticIncludesViteCommonJSHelper(t *testing.T) {
	data, err := Assets.ReadFile("static/app/assets/_commonjsHelpers.js")
	if err != nil {
		t.Fatalf("expected embedded Vite helper chunk: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected embedded Vite helper chunk to be non-empty")
	}
}
