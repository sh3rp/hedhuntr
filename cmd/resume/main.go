package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"hedhuntr/internal/document"
	"hedhuntr/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "import":
		if err := runImport(os.Args[2:], logger); err != nil {
			logger.Error("import resume", "error", err)
			os.Exit(1)
		}
	case "list":
		if err := runList(os.Args[2:]); err != nil {
			logger.Error("list resumes", "error", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func runImport(args []string, logger *slog.Logger) error {
	flags := flag.NewFlagSet("import", flag.ExitOnError)
	var dbPath string
	var documentDir string
	var resumePath string
	var name string
	var candidateProfileID int64
	flags.StringVar(&dbPath, "db", "hedhuntr.db", "SQLite database path")
	flags.StringVar(&documentDir, "documents", "data/documents", "document storage directory")
	flags.StringVar(&resumePath, "file", "examples/resume.example.md", "resume Markdown file to import")
	flags.StringVar(&name, "name", "", "resume source name")
	flags.Int64Var(&candidateProfileID, "candidate-profile-id", 0, "optional candidate profile ID")
	if err := flags.Parse(args); err != nil {
		return err
	}

	content, err := os.ReadFile(resumePath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(content)) == "" {
		return fmt.Errorf("resume file is empty")
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(resumePath), filepath.Ext(resumePath))
	}

	stored, err := document.StoreContent(documentDir, "resume_sources", filepath.Base(resumePath), content)
	if err != nil {
		return err
	}

	ctx := context.Background()
	st, err := store.Open(ctx, dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	documentID, err := st.CreateDocument(ctx, store.CreateDocumentParams{
		Kind:      "resume_source",
		Format:    "markdown",
		Path:      stored.Path,
		SHA256:    stored.SHA256,
		SizeBytes: stored.SizeBytes,
	})
	if err != nil {
		return err
	}

	var profileID *int64
	if candidateProfileID > 0 {
		profileID = &candidateProfileID
	}
	resumeID, err := st.CreateResumeSource(ctx, store.CreateResumeSourceParams{
		CandidateProfileID: profileID,
		Name:               name,
		Format:             "markdown",
		DocumentID:         documentID,
	})
	if err != nil {
		return err
	}

	logger.Info("resume imported", "resume_source_id", resumeID, "document_id", documentID, "path", stored.Path)
	return nil
}

func runList(args []string) error {
	flags := flag.NewFlagSet("list", flag.ExitOnError)
	var dbPath string
	flags.StringVar(&dbPath, "db", "hedhuntr.db", "SQLite database path")
	if err := flags.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()
	st, err := store.Open(ctx, dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	sources, err := st.ListResumeSources(ctx)
	if err != nil {
		return err
	}
	for _, source := range sources {
		fmt.Printf("%d\t%s\t%s\t%s\n", source.ID, source.Name, source.Format, source.DocumentPath)
	}
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/resume import -file examples/resume.example.md [-db hedhuntr.db] [-documents data/documents]")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/resume list [-db hedhuntr.db]")
}
