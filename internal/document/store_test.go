package document

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreContentCreatesDocumentPath(t *testing.T) {
	root := t.TempDir()
	stored, err := StoreContent(root, "resume_sources", "My Resume.md", []byte("# Resume\n"))
	if err != nil {
		t.Fatal(err)
	}
	if stored.SHA256 == "" {
		t.Fatal("SHA256 is empty")
	}
	if stored.SizeBytes != int64(len("# Resume\n")) {
		t.Fatalf("SizeBytes = %d, want %d", stored.SizeBytes, len("# Resume\n"))
	}
	if !strings.HasPrefix(stored.Path, filepath.Join(root, "resume_sources")) {
		t.Fatalf("Path = %q, want under resume_sources", stored.Path)
	}
	content, err := os.ReadFile(stored.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "# Resume\n" {
		t.Fatalf("content = %q, want resume content", string(content))
	}
}
