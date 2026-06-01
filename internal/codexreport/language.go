package codexreport

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxLanguageFiles = 4000

var languageByExt = map[string]string{
	".go":     "Go",
	".py":     "Python",
	".js":     "JavaScript",
	".jsx":    "JavaScript",
	".ts":     "TypeScript",
	".tsx":    "TypeScript",
	".rs":     "Rust",
	".java":   "Java",
	".kt":     "Kotlin",
	".kts":    "Kotlin",
	".swift":  "Swift",
	".rb":     "Ruby",
	".php":    "PHP",
	".cs":     "C#",
	".c":      "C",
	".h":      "C/C++",
	".cc":     "C++",
	".cpp":    "C++",
	".cxx":    "C++",
	".hpp":    "C++",
	".m":      "Objective-C",
	".mm":     "Objective-C++",
	".scala":  "Scala",
	".sc":     "Scala",
	".dart":   "Dart",
	".ex":     "Elixir",
	".exs":    "Elixir",
	".erl":    "Erlang",
	".hrl":    "Erlang",
	".clj":    "Clojure",
	".cljs":   "Clojure",
	".fs":     "F#",
	".fsx":    "F#",
	".r":      "R",
	".lua":    "Lua",
	".pl":     "Perl",
	".pm":     "Perl",
	".sh":     "Shell",
	".bash":   "Shell",
	".zsh":    "Shell",
	".fish":   "Shell",
	".sql":    "SQL",
	".html":   "HTML",
	".css":    "CSS",
	".scss":   "CSS",
	".sass":   "CSS",
	".vue":    "Vue",
	".svelte": "Svelte",
}

var languageByFile = map[string]string{
	"go.mod":           "Go",
	"package.json":     "JavaScript",
	"tsconfig.json":    "TypeScript",
	"pyproject.toml":   "Python",
	"requirements.txt": "Python",
	"cargo.toml":       "Rust",
	"gemfile":          "Ruby",
	"composer.json":    "PHP",
	"pom.xml":          "Java",
	"build.gradle":     "Java",
	"mix.exs":          "Elixir",
}

var ignoredLanguageDirs = map[string]bool{
	".git": true, ".hg": true, ".svn": true,
	"node_modules": true, "vendor": true, "dist": true, "build": true,
	"target": true, ".next": true, ".nuxt": true, ".cache": true,
	"coverage": true, ".venv": true, "venv": true, "__pycache__": true,
}

type projectProfile struct {
	Root         string
	Language     string
	LanguageMix  []LanguageCount
	BuildSystems []string
}

func inferLanguage(cwd string) string {
	return inferProjectProfile(cwd).Language
}

func inferProjectProfile(cwd string) projectProfile {
	root := repoRoot(cwd)
	if root == "" {
		return projectProfile{Language: "(unknown)"}
	}

	counts := map[string]int{}
	seen := 0
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
		if seen >= maxLanguageFiles {
			return filepath.SkipAll
		}
		seen++

		name := strings.ToLower(d.Name())
		if lang := languageByFile[name]; lang != "" {
			counts[lang]++
			return nil
		}
		if lang := languageByExt[strings.ToLower(filepath.Ext(name))]; lang != "" {
			counts[lang]++
		}
		return nil
	})

	mix := languageMix(counts, 5)
	best := "(unknown)"
	if len(mix) > 0 {
		best = mix[0].Language
	}
	return projectProfile{
		Root:         root,
		Language:     best,
		LanguageMix:  mix,
		BuildSystems: detectBuildSystems(root),
	}
}

func languageMix(counts map[string]int, limit int) []LanguageCount {
	var out []LanguageCount
	for lang, count := range counts {
		if count > 0 {
			out = append(out, LanguageCount{Language: lang, Files: count})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Files != out[j].Files {
			return out[i].Files > out[j].Files
		}
		return out[i].Language < out[j].Language
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func detectBuildSystems(root string) []string {
	if root == "" {
		return nil
	}
	checks := []struct {
		path string
		name string
	}{
		{"go.mod", "Go modules"},
		{"package.json", "npm/package.json"},
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "Yarn"},
		{"bun.lockb", "Bun"},
		{"Cargo.toml", "Cargo"},
		{"pyproject.toml", "Python/pyproject"},
		{"requirements.txt", "pip requirements"},
		{"pom.xml", "Maven"},
		{"build.gradle", "Gradle"},
		{"build.gradle.kts", "Gradle"},
		{"Makefile", "Make"},
		{"justfile", "Just"},
		{"Dockerfile", "Docker"},
		{"docker-compose.yml", "Docker Compose"},
		{"docker-compose.yaml", "Docker Compose"},
		{"CMakeLists.txt", "CMake"},
	}

	seen := map[string]bool{}
	var out []string
	for _, c := range checks {
		if _, err := os.Stat(filepath.Join(root, c.path)); err == nil && !seen[c.name] {
			seen[c.name] = true
			out = append(out, c.name)
		}
	}
	if entries, err := os.ReadDir(filepath.Join(root, ".github", "workflows")); err == nil && len(entries) > 0 {
		out = append(out, "GitHub Actions")
	}
	sort.Strings(out)
	return out
}

func fileChangeType(path string) string {
	base := strings.ToLower(filepath.Base(path))
	switch base {
	case "dockerfile":
		return "Dockerfile"
	case "makefile":
		return "Makefile"
	case "go.mod", "go.sum", "package.json", "tsconfig.json", "pyproject.toml", "cargo.toml", "pom.xml":
		return "Project config"
	}
	ext := strings.ToLower(filepath.Ext(base))
	if lang := languageByExt[ext]; lang != "" {
		return lang
	}
	switch ext {
	case ".md", ".mdx":
		return "Markdown"
	case ".json":
		return "JSON"
	case ".yaml", ".yml":
		return "YAML"
	case ".toml":
		return "TOML"
	case ".lock":
		return "Lockfile"
	case ".txt":
		return "Text"
	}
	return "(other)"
}

func repoRoot(cwd string) string {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		return ""
	}
	info, err := os.Stat(cwd)
	if err != nil {
		return ""
	}
	if !info.IsDir() {
		cwd = filepath.Dir(cwd)
	}
	dir, err := filepath.Abs(cwd)
	if err != nil {
		return ""
	}
	start := dir
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return start
		}
		dir = parent
	}
}
