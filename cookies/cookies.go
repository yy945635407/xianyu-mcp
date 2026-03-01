package cookies

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

type Cookier interface {
	LoadCookies() ([]byte, error)
	SaveCookies(data []byte) error
	DeleteCookies() error
}

type localCookie struct {
	path string
}

func NewLoadCookie(path string) Cookier {
	if path == "" {
		panic("path is required")
	}

	return &localCookie{path: path}
}

func (c *localCookie) LoadCookies() ([]byte, error) {
	data, err := os.ReadFile(c.path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read cookies from file")
	}
	return data, nil
}

func (c *localCookie) SaveCookies(data []byte) error {
	return os.WriteFile(c.path, data, 0644)
}

func (c *localCookie) DeleteCookies() error {
	if _, err := os.Stat(c.path); os.IsNotExist(err) {
		return nil
	}
	return os.Remove(c.path)
}

func GetCookiesFilePath() string {
	tmpDir := os.TempDir()
	oldPath := filepath.Join(tmpDir, "cookies.json")
	if _, err := os.Stat(oldPath); err == nil {
		return oldPath
	}

	path := os.Getenv("COOKIES_PATH")
	if path == "" {
		path = "cookies.json"
	}
	return path
}
