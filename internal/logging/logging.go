package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Entry struct {
	Timestamp time.Time
	Provider  string
	Model     string
	Mode      string
	Result    string
	Message   string
}

type FileLogger struct {
	Path string
}

func DefaultPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "autofresh", "autofresh.log"), nil
}

func NewFileLogger(path string) *FileLogger {
	return &FileLogger{Path: path}
}

func (l *FileLogger) Log(entry Entry) error {
	if l == nil || l.Path == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(l.Path), 0o755); err != nil {
		return err
	}

	handle, err := os.OpenFile(l.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer handle.Close()

	line := fmt.Sprintf(
		"%s provider=%s model=%s mode=%s result=%s message=%q\n",
		entry.Timestamp.Format(time.RFC3339),
		entry.Provider,
		entry.Model,
		entry.Mode,
		entry.Result,
		entry.Message,
	)

	_, err = handle.WriteString(line)
	return err
}
