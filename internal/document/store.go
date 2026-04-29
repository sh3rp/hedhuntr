package document

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type StoredDocument struct {
	Path      string
	SHA256    string
	SizeBytes int64
}

func StoreContent(rootDir, kind, name string, content []byte) (StoredDocument, error) {
	if rootDir == "" {
		rootDir = "data/documents"
	}
	if kind == "" {
		return StoredDocument{}, fmt.Errorf("kind is required")
	}
	if name == "" {
		return StoredDocument{}, fmt.Errorf("name is required")
	}

	sum := sha256.Sum256(content)
	hash := hex.EncodeToString(sum[:])
	dir := filepath.Join(rootDir, kind, time.Now().UTC().Format("2006/01/02"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return StoredDocument{}, err
	}

	filename := safeFilename(strings.TrimSuffix(name, filepath.Ext(name))) + "-" + hash[:12] + filepath.Ext(name)
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return StoredDocument{}, err
	}

	return StoredDocument{
		Path:      path,
		SHA256:    hash,
		SizeBytes: int64(len(content)),
	}, nil
}

func safeFilename(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "document"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
