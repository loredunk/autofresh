package codexreport

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Thread is the per-session metadata we need to attribute rollout usage. It is
// sourced from state_*.sqlite's `threads` table when available, otherwise
// reconstructed from each rollout's session_meta during parsing.
type Thread struct {
	ID           string
	RolloutPath  string
	CreatedAt    int64 // unix seconds
	Source       string
	GitBranch    string
	GitOriginURL string
	Cwd          string
	Model        string
	IsSubagent   bool
}

// CodexHome returns the active CODEX_HOME, defaulting to ~/.codex.
func CodexHome() (string, error) {
	if v := strings.TrimSpace(os.Getenv("CODEX_HOME")); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex"), nil
}

// discoverThreads returns the set of non-subagent threads from the newest
// readable state_*.sqlite. The bool reports whether sqlite was usable; on false
// the caller should fall back to globbing the sessions tree.
func discoverThreads(codexHome string) ([]Thread, bool) {
	dbPath := newestStateDB(codexHome)
	if dbPath == "" {
		return nil, false
	}
	// A live WAL means the main file may be stale; we don't merge WAL, so bail
	// to the glob fallback rather than report wrong numbers.
	if _, err := os.Stat(dbPath + "-wal"); err == nil {
		return nil, false
	}
	threads, err := readThreads(dbPath)
	if err != nil || len(threads) == 0 {
		return nil, false
	}
	return threads, true
}

// newestStateDB picks the most recently modified state_*.sqlite in CODEX_HOME.
func newestStateDB(codexHome string) string {
	matches, _ := filepath.Glob(filepath.Join(codexHome, "state_*.sqlite"))
	best := ""
	var bestMod int64
	for _, m := range matches {
		fi, err := os.Stat(m)
		if err != nil {
			continue
		}
		if mod := fi.ModTime().Unix(); best == "" || mod > bestMod {
			best, bestMod = m, mod
		}
	}
	return best
}

func readThreads(dbPath string) ([]Thread, error) {
	db, err := openSQLite(dbPath)
	if err != nil {
		return nil, err
	}
	cols, err := db.tableColumns("threads")
	if err != nil {
		return nil, err
	}
	idx := map[string]int{}
	for i, c := range cols {
		idx[c] = i
	}
	root, err := db.findTableRoot("threads")
	if err != nil {
		return nil, err
	}
	rows, err := db.scanTable(root, false)
	if err != nil {
		return nil, err
	}

	get := func(rec []any, name string) any {
		i, ok := idx[name]
		if !ok || i >= len(rec) {
			return nil
		}
		return rec[i]
	}

	var out []Thread
	for _, rec := range rows {
		source := asString(get(rec, "source"))
		t := Thread{
			ID:           asString(get(rec, "id")),
			RolloutPath:  asString(get(rec, "rollout_path")),
			CreatedAt:    asInt(get(rec, "created_at")),
			Source:       source,
			GitBranch:    asString(get(rec, "git_branch")),
			GitOriginURL: asString(get(rec, "git_origin_url")),
			Cwd:          asString(get(rec, "cwd")),
			Model:        asString(get(rec, "model")),
			IsSubagent:   isSubagentSource(source),
		}
		if t.RolloutPath == "" {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

// isSubagentSource detects spawned sub-agent threads. Their `source` is a JSON
// blob like {"subagent":{"thread_spawn":{...}}}; their rollouts replay the
// parent's token history and must be excluded to avoid massive over-counting.
func isSubagentSource(source string) bool {
	return strings.Contains(source, "subagent") || strings.Contains(source, "thread_spawn")
}

// tableColumns parses the stored CREATE TABLE statement to recover column names
// in storage order (including ALTER-appended columns, which SQLite keeps in the
// same sql text).
func (db *sqliteDB) tableColumns(name string) ([]string, error) {
	rows, err := db.scanTable(1, true)
	if err != nil {
		return nil, err
	}
	for _, rec := range rows {
		if len(rec) < 5 {
			continue
		}
		if asString(rec[0]) == "table" && asString(rec[1]) == name {
			return parseColumnNames(asString(rec[4]))
		}
	}
	return nil, fmt.Errorf("schema for %q not found", name)
}

// parseColumnNames extracts column names from a CREATE TABLE body, splitting the
// top-level parenthesised list on commas and taking the first identifier of each
// definition (skipping table-level constraints).
func parseColumnNames(sql string) ([]string, error) {
	open := strings.Index(sql, "(")
	if open < 0 {
		return nil, fmt.Errorf("malformed create sql")
	}
	body := sql[open+1:]
	var defs []string
	depth := 0
	start := 0
	for i, r := range body {
		switch r {
		case '(':
			depth++
		case ')':
			if depth == 0 {
				defs = append(defs, body[start:i])
				start = -1
			} else {
				depth--
			}
		case ',':
			if depth == 0 {
				defs = append(defs, body[start:i])
				start = i + 1
			}
		}
		if start == -1 {
			break
		}
	}
	constraintKW := map[string]bool{
		"primary": true, "unique": true, "check": true,
		"foreign": true, "constraint": true,
	}
	var cols []string
	for _, d := range defs {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		first := strings.Fields(d)[0]
		first = strings.Trim(first, "\"`[]'")
		if constraintKW[strings.ToLower(first)] {
			continue
		}
		cols = append(cols, first)
	}
	return cols, nil
}

// globRollouts walks CODEX_HOME/sessions for rollout-*.jsonl files whose date
// directory falls within [from, to] (inclusive, by local date). It is the
// fallback when sqlite is unavailable; subagent detection then happens during
// parsing via session_meta.
func globRollouts(codexHome string) ([]string, error) {
	root := filepath.Join(codexHome, "sessions")
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // tolerate unreadable dirs
		}
		if info.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if strings.HasPrefix(base, "rollout-") && strings.HasSuffix(base, ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}
