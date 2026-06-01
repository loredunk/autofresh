package codexreport

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var codexConfigKeys = map[string]bool{
	"approval_policy":                true,
	"model":                          true,
	"model_provider":                 true,
	"model_reasoning_effort":         true,
	"model_reasoning_summary":        true,
	"model_verbosity":                true,
	"model_context_window":           true,
	"model_auto_compact_token_limit": true,
	"project_doc_max_bytes":          true,
	"project_doc_fallback_filenames": true,
	"sandbox_mode":                   true,
	"web_search":                     true,
	"preferred_auth_method":          true,
}

var sensitiveConfigKeyParts = []string{
	"api_key",
	"bearer_token",
	"token",
	"secret",
	"password",
	"authorization",
	"http_headers",
}

func scanCodexConfig(codexHome string, agg *aggregate) CodexConfigReport {
	var out CodexConfigReport
	out.UserConfig = scanConfigFile(filepath.Join(codexHome, "config.toml"), codexHome)
	out.ProfileConfigs = scanProfileConfigs(codexHome)
	out.GlobalInstructionFiles = scanInstructionFiles(codexHome, codexHome)
	out.ProjectInstructionFiles = countProjectInstructionFiles(agg)
	out.HistoryPersistence = inferHistoryPersistence(out.UserConfig)
	return out
}

func scanProfileConfigs(codexHome string) []ConfigSummary {
	matches, _ := filepath.Glob(filepath.Join(codexHome, "*.config.toml"))
	sort.Strings(matches)
	var out []ConfigSummary
	for _, path := range matches {
		out = append(out, scanConfigFile(path, codexHome))
	}
	return out
}

func scanConfigFile(path, base string) ConfigSummary {
	s := ConfigSummary{Path: displayPath(path, base)}
	f, err := os.Open(path)
	if err != nil {
		return s
	}
	defer f.Close()

	s.Exists = true
	s.Keys = map[string]string{}
	seenSecrets := map[string]bool{}
	section := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.Contains(line, "]") {
			section = strings.TrimSpace(strings.Trim(line, "[]"))
			if strings.HasPrefix(section, "mcp_servers.") {
				name := strings.TrimPrefix(section, "mcp_servers.")
				s.Keys["mcp_server:"+strings.Trim(name, `"'`)] = "configured"
			}
			if strings.HasPrefix(section, "profiles.") {
				name := strings.TrimPrefix(section, "profiles.")
				s.Keys["profile:"+strings.Trim(name, `"'`)] = "configured"
			}
			continue
		}
		key, value, ok := splitConfigAssignment(line)
		if !ok {
			continue
		}
		fullKey := key
		if section != "" {
			fullKey = section + "." + key
		}
		if isSensitiveConfigKey(fullKey) {
			if !seenSecrets[fullKey] {
				seenSecrets[fullKey] = true
				s.Secrets = append(s.Secrets, fullKey)
			}
			continue
		}
		if codexConfigKeys[key] || strings.HasPrefix(fullKey, "memories.") ||
			strings.HasPrefix(fullKey, "history.") || strings.HasPrefix(fullKey, "shell_environment_policy.") {
			s.Keys[fullKey] = cleanConfigValue(value)
		}
	}
	sort.Strings(s.Secrets)
	if len(s.Keys) == 0 {
		s.Keys = nil
	}
	return s
}

func splitConfigAssignment(line string) (string, string, bool) {
	i := strings.Index(line, "=")
	if i < 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:i])
	value := strings.TrimSpace(stripInlineComment(line[i+1:]))
	if key == "" {
		return "", "", false
	}
	return key, value, true
}

func stripInlineComment(value string) string {
	inQuote := rune(0)
	for i, r := range value {
		switch r {
		case '\'', '"':
			if inQuote == 0 {
				inQuote = r
			} else if inQuote == r {
				inQuote = 0
			}
		case '#':
			if inQuote == 0 {
				return strings.TrimSpace(value[:i])
			}
		}
	}
	return value
}

func cleanConfigValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	if len(value) > 120 {
		return value[:119] + "…"
	}
	return value
}

func isSensitiveConfigKey(key string) bool {
	key = strings.ToLower(key)
	for _, part := range sensitiveConfigKeyParts {
		if strings.Contains(key, part) {
			return true
		}
	}
	return false
}

func scanInstructionFiles(root, base string) []InstructionSummary {
	names := []string{"AGENTS.override.md", "AGENTS.md"}
	var out []InstructionSummary
	for _, name := range names {
		path := filepath.Join(root, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() && info.Size() > 0 {
			out = append(out, InstructionSummary{Path: displayPath(path, base), Bytes: info.Size()})
		}
	}
	return out
}

func countProjectInstructionFiles(agg *aggregate) int {
	if agg == nil {
		return 0
	}
	seen := map[string]bool{}
	total := 0
	for _, repo := range agg.byRepo {
		if repo.root == "" || seen[repo.root] {
			continue
		}
		seen[repo.root] = true
		total += countInstructionFiles(repo.root)
	}
	return total
}

func countInstructionFiles(root string) int {
	count := 0
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && ignoredLanguageDirs[strings.ToLower(d.Name())] {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if name == "AGENTS.md" || name == "AGENTS.override.md" {
			if info, err := d.Info(); err == nil && info.Size() > 0 {
				count++
			}
		}
		return nil
	})
	return count
}

func inferHistoryPersistence(cfg ConfigSummary) string {
	if !cfg.Exists {
		return "unknown"
	}
	for key, value := range cfg.Keys {
		if key == "history.persistence" || key == "history.save" {
			return value
		}
	}
	return "default"
}

func displayPath(path, base string) string {
	if base != "" {
		if rel, err := filepath.Rel(base, path); err == nil && !strings.HasPrefix(rel, "..") {
			return rel
		}
	}
	home, err := os.UserHomeDir()
	if err == nil {
		if rel, err := filepath.Rel(home, path); err == nil && !strings.HasPrefix(rel, "..") {
			return "~/" + rel
		}
	}
	return path
}
