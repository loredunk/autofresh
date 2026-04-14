package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

var ErrNotFound = errors.New("autofresh config not found")

type Config struct {
	StartTime       string `json:"start_time"`
	Target          string `json:"target"`
	IntervalMinutes int    `json:"interval_minutes"`
	Timezone        string `json:"timezone"`
	BinaryPath      string `json:"binary_path"`
}

type Store struct {
	path string
}

func DefaultPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(root, "autofresh", "config.json"), nil
}

func NewStore(path string) Store {
	return Store{path: path}
}

func (s Store) Path() string {
	return s.path
}

func (s Store) Load() (Config, error) {
	return Load(s.path)
}

func (s Store) Save(cfg Config) error {
	return Save(s.path, cfg)
}

func (s Store) Delete() error {
	return Delete(s.path)
}

func (s Store) Exists() (bool, error) {
	return Exists(s.path)
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, ErrNotFound
		}
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func Delete(path string) error {
	err := os.Remove(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}
