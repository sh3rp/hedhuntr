package store

import (
	"context"
	"path/filepath"
	"testing"

	"hedhuntr/internal/document"
)

func TestResumeSourceCreateListLoad(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "resumes.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	stored, err := document.StoreContent(t.TempDir(), "resume_sources", "resume.md", []byte("# Resume\nGo engineer\n"))
	if err != nil {
		t.Fatal(err)
	}
	documentID, err := st.CreateDocument(ctx, CreateDocumentParams{
		Kind:      "resume_source",
		Format:    "markdown",
		Path:      stored.Path,
		SHA256:    stored.SHA256,
		SizeBytes: stored.SizeBytes,
	})
	if err != nil {
		t.Fatal(err)
	}
	resumeID, err := st.CreateResumeSource(ctx, CreateResumeSourceParams{
		Name:       "Base Resume",
		Format:     "markdown",
		DocumentID: documentID,
	})
	if err != nil {
		t.Fatal(err)
	}

	sources, err := st.ListResumeSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 {
		t.Fatalf("len(sources) = %d, want 1", len(sources))
	}
	if sources[0].Name != "Base Resume" {
		t.Fatalf("Name = %q, want Base Resume", sources[0].Name)
	}

	loaded, content, err := st.LoadResumeSourceContent(ctx, resumeID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DocumentID != documentID {
		t.Fatalf("DocumentID = %d, want %d", loaded.DocumentID, documentID)
	}
	if string(content) != "# Resume\nGo engineer\n" {
		t.Fatalf("content = %q, want resume content", string(content))
	}
}
