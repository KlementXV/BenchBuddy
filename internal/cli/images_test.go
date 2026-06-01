package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestImagesListCmd_Text(t *testing.T) {
	var out bytes.Buffer
	f := imagesListFlags{profile: "quick", format: "text", sourceRegistry: "ghcr.io/clementlevoux/benchbuddy"}
	if err := imagesListCmd(&out, f); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "runner") {
		t.Errorf("expected 'runner' in text output, got: %s", got)
	}
}

func TestImagesListCmd_JSON(t *testing.T) {
	var out bytes.Buffer
	f := imagesListFlags{profile: "quick", format: "json", sourceRegistry: "ghcr.io/clementlevoux/benchbuddy"}
	if err := imagesListCmd(&out, f); err != nil {
		t.Fatal(err)
	}
	var entries []imageEntry
	if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
		t.Fatalf("decode JSON: %v\noutput: %s", err, out.String())
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 image entry, got %d", len(entries))
	}
	if entries[0].Name != "runner" {
		t.Errorf("expected name=runner, got %q", entries[0].Name)
	}
}

func TestImagesListCmd_Script(t *testing.T) {
	var out bytes.Buffer
	f := imagesListFlags{profile: "quick", format: "script", sourceRegistry: "ghcr.io/clementlevoux/benchbuddy"}
	if err := imagesListCmd(&out, f); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "skopeo copy") {
		t.Errorf("expected 'skopeo copy' in script output, got: %s", got)
	}
}

func TestImagesListCmd_InvalidFormat(t *testing.T) {
	var out bytes.Buffer
	f := imagesListFlags{profile: "quick", format: "html", sourceRegistry: "ghcr.io/clementlevoux/benchbuddy"}
	if err := imagesListCmd(&out, f); err == nil {
		t.Error("expected error for unsupported format")
	}
}
