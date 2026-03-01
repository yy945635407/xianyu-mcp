package main

import (
	"os"
	"path/filepath"
)

func dataDir() string {
	if d := os.Getenv("DATA_DIR"); d != "" {
		return d
	}
	return "data"
}

func dataFilePath(filename string) string {
	return filepath.Join(dataDir(), filename)
}
