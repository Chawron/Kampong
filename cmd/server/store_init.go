package main

import (
	"log"

	"github.com/kampong/debate/internal/config"
	"github.com/kampong/debate/internal/store"
)

// initSessionStore picks the backend from config and runs any required
// migrations. Returns the Session interface; the concrete type may also
// implement EventStore / CalibrationStore which the rest of the program can
// type-assert as needed.
func initSessionStore(cfg *config.Config) (store.Session, error) {
	backend := cfg.Storage.Backend
	if backend == "" {
		backend = "json"
	}

	switch backend {
	case "sqlite":
		path := cfg.Storage.SQLitePath
		if path == "" {
			path = "data/kampong.db"
		}
		sqlStore, err := store.NewSQLiteStore(path)
		if err != nil {
			return nil, err
		}
		// One-shot importer: copy legacy JSON files into SQLite on first run.
		imported, skipped, err := sqlStore.ImportJSON("data/debates")
		if err != nil {
			log.Printf("SQLITE IMPORT WARN: %v", err)
		} else if imported > 0 || skipped > 0 {
			log.Printf("SQLITE IMPORT: %d imported, %d skipped (already present or unreadable)", imported, skipped)
		}
		log.Printf("Session store: sqlite (%s)", path)
		return sqlStore, nil

	default: // "json" or any unknown value falls back to JSON.
		jsonStore, err := store.NewSessionStore("data/debates")
		if err != nil {
			return nil, err
		}
		log.Println("Session store: json (data/debates/)")
		return jsonStore, nil
	}
}