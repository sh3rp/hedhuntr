package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"hedhuntr/internal/profile"
	"hedhuntr/internal/store"
)

func main() {
	var dbPath string
	var profilePath string
	var printJSON bool
	flag.StringVar(&dbPath, "db", "hedhuntr.db", "SQLite database path")
	flag.StringVar(&profilePath, "profile", "configs/candidate-profile.example.json", "candidate profile JSON path")
	flag.BoolVar(&printJSON, "print", false, "print the stored profile after import")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	content, err := os.ReadFile(profilePath)
	if err != nil {
		logger.Error("read profile", "error", err, "path", profilePath)
		os.Exit(1)
	}

	var candidate profile.Profile
	if err := json.Unmarshal(content, &candidate); err != nil {
		logger.Error("decode profile", "error", err, "path", profilePath)
		os.Exit(1)
	}
	if err := profile.Validate(candidate); err != nil {
		logger.Error("validate profile", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	st, err := store.Open(ctx, dbPath)
	if err != nil {
		logger.Error("open store", "error", err, "db", dbPath)
		os.Exit(1)
	}
	defer st.Close()

	id, err := st.UpsertFullCandidateProfile(ctx, candidate)
	if err != nil {
		logger.Error("store profile", "error", err)
		os.Exit(1)
	}

	logger.Info("profile imported", "profile_id", id, "name", candidate.Name)
	if printJSON {
		stored, err := st.LoadFullCandidateProfile(ctx, id)
		if err != nil {
			logger.Error("load stored profile", "error", err, "profile_id", id)
			os.Exit(1)
		}
		encoded, err := json.MarshalIndent(stored, "", "  ")
		if err != nil {
			logger.Error("encode stored profile", "error", err)
			os.Exit(1)
		}
		fmt.Println(string(encoded))
	}
}
