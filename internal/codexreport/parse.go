package codexreport

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// tokenUsage mirrors info.total_token_usage in token_count events. Values are
// cumulative per thread, so we diff consecutive samples to get increments.
type tokenUsage struct {
	Input           int64 `json:"input_tokens"`
	CachedInput     int64 `json:"cached_input_tokens"`
	Output          int64 `json:"output_tokens"`
	ReasoningOutput int64 `json:"reasoning_output_tokens"`
	Total           int64 `json:"total_tokens"`
}

func (a tokenUsage) sub(b tokenUsage) tokenUsage {
	return tokenUsage{
		Input:           a.Input - b.Input,
		CachedInput:     a.CachedInput - b.CachedInput,
		Output:          a.Output - b.Output,
		ReasoningOutput: a.ReasoningOutput - b.ReasoningOutput,
		Total:           a.Total - b.Total,
	}
}

func (a *tokenUsage) add(b tokenUsage) {
	a.Input += b.Input
	a.CachedInput += b.CachedInput
	a.Output += b.Output
	a.ReasoningOutput += b.ReasoningOutput
	a.Total += b.Total
}

// rolloutLine is the minimal envelope shared by every JSONL record.
type rolloutLine struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

// aggregate accumulates everything the report needs across all parsed threads.
type aggregate struct {
	tokens tokenUsage
	cost   float64

	shellCalls    int
	webSearches   int
	fileChanges   int // distinct file-change operations from patch_apply_end
	totalCalls    int
	shellCommands map[string]int // binary name -> count
	gitCommands   map[string]int // normalized git subcommand -> count

	sessionIDs map[string]bool
	duration   time.Duration

	byRepo       map[string]*repoAgg
	byHour       [24]tokenUsage
	bySource     map[string]*usageAgg
	byLanguage   map[string]*usageAgg
	profileCache map[string]projectProfile

	missingPrice map[string]bool // models we had no price for
	modelsSeen   map[string]bool
}

type repoAgg struct {
	repo         string
	branches     map[string]bool
	tokens       tokenUsage
	cost         float64
	sessions     map[string]bool
	root         string
	language     string
	languageMix  []LanguageCount
	buildSystems map[string]bool
	testCommands map[string]bool
	fileTypes    map[string]int
}

type usageAgg struct {
	name     string
	tokens   tokenUsage
	sessions map[string]bool
}

func newAggregate() *aggregate {
	return &aggregate{
		shellCommands: map[string]int{},
		gitCommands:   map[string]int{},
		sessionIDs:    map[string]bool{},
		byRepo:        map[string]*repoAgg{},
		bySource:      map[string]*usageAgg{},
		byLanguage:    map[string]*usageAgg{},
		profileCache:  map[string]projectProfile{},
		missingPrice:  map[string]bool{},
		modelsSeen:    map[string]bool{},
	}
}

// parseOptions controls the date window and timezone for attribution.
type parseOptions struct {
	from time.Time // inclusive, local
	to   time.Time // exclusive, local
	loc  *time.Location
}

// inRange reports whether a UTC rollout timestamp falls in the local window.
func (o parseOptions) inRange(ts time.Time) bool {
	local := ts.In(o.loc)
	return !local.Before(o.from) && local.Before(o.to)
}

// parseThread reads one rollout file and folds its in-window usage into agg.
// meta carries repo/branch/model context for attribution (may be partially
// empty in the glob fallback path).
func parseThread(meta Thread, opts parseOptions, agg *aggregate) {
	f, err := os.Open(meta.RolloutPath)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	sessionID := meta.ID
	repo := repoKey(meta)
	branch := meta.GitBranch
	curModel := meta.Model
	source := sourceKey(meta.Source)
	profile := agg.profileForCWD(meta.Cwd)

	var haveBaseline bool
	var baseline tokenUsage
	threadTouched := false
	var prevActive time.Time // last in-window token_count, for active-time gaps

	for sc.Scan() {
		var line rolloutLine
		if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
			continue
		}
		ts, _ := time.Parse(time.RFC3339Nano, line.Timestamp)

		switch line.Type {
		case "session_meta":
			// Fallback path: recover ids/cwd when sqlite metadata was absent.
			var sm struct {
				ID            string `json:"id"`
				Cwd           string `json:"cwd"`
				Source        string `json:"source"`
				ModelProvider string `json:"model_provider"`
			}
			_ = json.Unmarshal(line.Payload, &sm)
			if sessionID == "" {
				sessionID = sm.ID
			}
			if source == "(unknown)" && sm.Source != "" {
				source = sourceKey(sm.Source)
			}
			if (meta.Cwd == "" || repo == "(unknown)") && sm.Cwd != "" {
				meta.Cwd = sm.Cwd
				repo = repoKey(meta)
				profile = agg.profileForCWD(meta.Cwd)
			}

		case "turn_context":
			var tc struct {
				Model string `json:"model"`
			}
			if json.Unmarshal(line.Payload, &tc) == nil && tc.Model != "" {
				curModel = tc.Model
			}

		case "event_msg":
			var pm struct {
				Type string `json:"type"`
				Info *struct {
					TotalTokenUsage tokenUsage `json:"total_token_usage"`
				} `json:"info"`
				Changes map[string]json.RawMessage `json:"changes"`
			}
			if json.Unmarshal(line.Payload, &pm) != nil {
				continue
			}
			switch pm.Type {
			case "token_count":
				// info==null is a rate-limit-only refresh; skip it so we don't
				// treat a repeated sample as a new increment.
				if pm.Info == nil {
					continue
				}
				cur := pm.Info.TotalTokenUsage
				if !haveBaseline {
					baseline, haveBaseline = cur, true
					continue
				}
				delta := cur.sub(baseline)
				baseline = cur
				// Compaction/rollback can lower the cumulative counters; ignore
				// negative deltas rather than subtract usage.
				if delta.Total < 0 || delta.Input < 0 || delta.Output < 0 {
					continue
				}
				if !opts.inRange(ts) {
					continue
				}
				agg.applyTokens(delta, curModel, repo, branch, sessionID, source, profile, ts, opts.loc)
				threadTouched = true
				// Active time: accumulate consecutive gaps, treating any pause
				// longer than idleCap as the session being idle (resumed later,
				// possibly days apart) rather than continuous work.
				if !prevActive.IsZero() {
					if gap := ts.Sub(prevActive); gap > 0 && gap <= idleCap {
						agg.duration += gap
					}
				}
				prevActive = ts
			case "patch_apply_end":
				if opts.inRange(ts) {
					agg.fileChanges += len(pm.Changes)
					agg.applyFileChanges(repo, branch, sessionID, profile, pm.Changes)
				}
			}

		case "response_item":
			var pm struct {
				Type string `json:"type"`
				Name string `json:"name"`
				Args string `json:"arguments"`
			}
			if json.Unmarshal(line.Payload, &pm) != nil {
				continue
			}
			if !opts.inRange(ts) {
				continue
			}
			switch pm.Type {
			case "function_call":
				agg.totalCalls++
				switch pm.Name {
				case "exec_command", "local_shell_call", "shell":
					agg.shellCalls++
					if cmd := firstCommand(pm.Args); cmd != "" {
						agg.shellCommands[cmd]++
					}
					if git := gitSubcommand(normalizedCommandLine(pm.Args)); git != "" {
						agg.gitCommands[git]++
					}
					if cmd := normalizedCommandLine(pm.Args); isTestCommand(cmd) {
						agg.applyTestCommand(repo, branch, sessionID, profile, cmd)
					}
				}
			case "local_shell_call":
				agg.totalCalls++
				agg.shellCalls++
			case "custom_tool_call":
				agg.totalCalls++
			case "web_search_call":
				agg.totalCalls++
				agg.webSearches++
			case "image_generation_call":
				agg.totalCalls++
			}
		}
	}

	if threadTouched {
		agg.sessionIDs[sessionID] = true
	}
}

// idleCap bounds the gap between two token_count events that still counts as
// continuous active work. Larger gaps are treated as idle/resumed sessions.
const idleCap = 5 * time.Minute

func (agg *aggregate) applyTokens(d tokenUsage, model, repo, branch, session, source string, profile projectProfile, ts time.Time, loc *time.Location) {
	agg.tokens.add(d)
	cost, priced := estimateCost(d, model)
	agg.cost += cost
	agg.modelsSeen[normalizeModel(model)] = true
	if !priced && model != "" {
		agg.missingPrice[normalizeModel(model)] = true
	}

	r := agg.repoFor(repo, profile)
	r.tokens.add(d)
	r.cost += cost
	r.sessions[session] = true
	if branch != "" {
		r.branches[branch] = true
	}

	agg.applyUsage(agg.bySource, source, d, session)
	agg.applyUsage(agg.byLanguage, profile.Language, d, session)

	hour := ts.In(loc).Hour()
	agg.byHour[hour].add(d)
}

func (agg *aggregate) repoFor(repo string, profile projectProfile) *repoAgg {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		repo = "(unknown)"
	}
	r := agg.byRepo[repo]
	if r == nil {
		r = &repoAgg{
			repo:         repo,
			branches:     map[string]bool{},
			sessions:     map[string]bool{},
			buildSystems: map[string]bool{},
			testCommands: map[string]bool{},
			fileTypes:    map[string]int{},
		}
		agg.byRepo[repo] = r
	}
	r.applyProfile(profile)
	return r
}

func (r *repoAgg) applyProfile(profile projectProfile) {
	if profile.Root != "" {
		r.root = profile.Root
	}
	if profile.Language != "" && profile.Language != "(unknown)" {
		r.language = profile.Language
	}
	if len(profile.LanguageMix) > 0 {
		r.languageMix = profile.LanguageMix
	}
	for _, system := range profile.BuildSystems {
		r.buildSystems[system] = true
	}
}

func (agg *aggregate) applyFileChanges(repo, branch, session string, profile projectProfile, changes map[string]json.RawMessage) {
	r := agg.repoFor(repo, profile)
	if session != "" {
		r.sessions[session] = true
	}
	if branch != "" {
		r.branches[branch] = true
	}
	for path := range changes {
		r.fileTypes[fileChangeType(path)]++
	}
}

func (agg *aggregate) applyTestCommand(repo, branch, session string, profile projectProfile, command string) {
	command = strings.TrimSpace(command)
	if command == "" {
		return
	}
	r := agg.repoFor(repo, profile)
	if session != "" {
		r.sessions[session] = true
	}
	if branch != "" {
		r.branches[branch] = true
	}
	r.testCommands[truncateCommand(command, 120)] = true
}

func (agg *aggregate) applyUsage(groups map[string]*usageAgg, name string, d tokenUsage, session string) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "(unknown)"
	}
	g := groups[name]
	if g == nil {
		g = &usageAgg{name: name, sessions: map[string]bool{}}
		groups[name] = g
	}
	g.tokens.add(d)
	g.sessions[session] = true
}

func sourceKey(source string) string {
	s := strings.ToLower(strings.TrimSpace(source))
	switch {
	case s == "":
		return "(unknown)"
	case strings.Contains(s, "vscode") || strings.Contains(s, "ide"):
		return "plugin"
	case strings.Contains(s, "codex-app") || strings.Contains(s, "desktop") || s == "app":
		return "codex-app"
	case strings.Contains(s, "cli") || strings.Contains(s, "terminal"):
		return "cli"
	default:
		return s
	}
}

func (agg *aggregate) profileForCWD(cwd string) projectProfile {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		return projectProfile{Language: "(unknown)"}
	}
	if profile, ok := agg.profileCache[cwd]; ok {
		return profile
	}
	profile := inferProjectProfile(cwd)
	agg.profileCache[cwd] = profile
	return profile
}

// repoKey derives a stable grouping key: the git origin repo name if known,
// otherwise the working-directory basename.
func repoKey(t Thread) string {
	if t.GitOriginURL != "" {
		return repoNameFromURL(t.GitOriginURL)
	}
	if t.Cwd != "" {
		return filepath.Base(t.Cwd)
	}
	return "(unknown)"
}

func repoNameFromURL(url string) string {
	url = strings.TrimSpace(url)
	url = strings.TrimSuffix(url, ".git")
	url = strings.TrimSuffix(url, "/")
	if i := strings.LastIndexAny(url, "/:"); i >= 0 {
		return url[i+1:]
	}
	return url
}

// firstCommand extracts the invoked binary name from an exec_command arguments
// JSON blob (e.g. {"cmd":"rg -n foo","workdir":"..."}). Persistence in limited
// mode may drop fields, so we tolerate missing/garbage input.
func firstCommand(args string) string {
	cmd := normalizedCommandLine(args)
	if cmd == "" {
		return ""
	}
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return ""
	}
	head := strings.Trim(fields[0], "\"'`")
	if head == "" {
		return ""
	}
	base := filepath.Base(head)
	// Strip an env-style leading assignment like FOO=bar.
	if strings.Contains(base, "=") {
		return "env"
	}
	return base
}

func commandLine(args string) string {
	if args == "" {
		return ""
	}
	var a struct {
		Cmd     string   `json:"cmd"`
		Command []string `json:"command"`
	}
	if json.Unmarshal([]byte(args), &a) != nil {
		return ""
	}
	if a.Cmd != "" {
		return strings.TrimSpace(a.Cmd)
	}
	if len(a.Command) > 0 {
		return strings.TrimSpace(strings.Join(a.Command, " "))
	}
	return ""
}

func normalizedCommandLine(args string) string {
	return strings.Join(stripShellWrapper(strings.Fields(commandLine(args))), " ")
}

func stripShellWrapper(fields []string) []string {
	for len(fields) > 0 {
		head := strings.Trim(fields[0], "\"'`")
		if head == "bash" || head == "sh" || head == "zsh" || head == "fish" ||
			head == "-lc" || head == "-c" || head == "env" {
			fields = fields[1:]
			continue
		}
		break
	}
	return fields
}

func isTestCommand(command string) bool {
	fields := strings.Fields(strings.ToLower(command))
	if len(fields) == 0 {
		return false
	}
	head := filepath.Base(strings.Trim(fields[0], "\"'`"))
	joined := " " + strings.Join(fields, " ") + " "
	switch head {
	case "go":
		return len(fields) > 1 && fields[1] == "test"
	case "cargo":
		return strings.Contains(joined, " test ") || strings.Contains(joined, " nextest ")
	case "npm", "pnpm", "yarn", "bun":
		return strings.Contains(joined, " test") || strings.Contains(joined, " vitest") || strings.Contains(joined, " jest")
	case "pytest", "tox", "jest", "vitest", "mocha", "ctest":
		return true
	case "python", "python3":
		return strings.Contains(joined, " -m pytest ") || strings.Contains(joined, " -m unittest ")
	case "make", "just":
		return containsTestLikeTarget(fields[1:])
	case "mvn", "gradle", "gradlew":
		return strings.Contains(joined, " test ") || strings.Contains(joined, " check ")
	default:
		return strings.HasSuffix(head, "gradlew") &&
			(strings.Contains(joined, " test ") || strings.Contains(joined, " check "))
	}
}

func gitSubcommand(command string) string {
	fields := strings.Fields(strings.ToLower(command))
	if len(fields) < 2 || filepath.Base(strings.Trim(fields[0], "\"'`")) != "git" {
		return ""
	}
	for _, field := range fields[1:] {
		field = strings.Trim(field, "\"'`")
		if field == "" {
			continue
		}
		if strings.HasPrefix(field, "-") {
			continue
		}
		return field
	}
	return ""
}

func containsTestLikeTarget(targets []string) bool {
	for _, target := range targets {
		target = strings.Trim(target, "\"'`")
		if target == "test" || target == "check" || target == "ci" ||
			strings.Contains(target, "test") || strings.Contains(target, "check") {
			return true
		}
	}
	return false
}

func truncateCommand(command string, n int) string {
	command = strings.Join(strings.Fields(command), " ")
	if len(command) <= n {
		return command
	}
	return command[:n-1] + "…"
}

// loadThreadsForRange returns the thread set to parse, preferring sqlite and
// falling back to globbing the sessions tree. Subagent threads are excluded.
func loadThreadsForRange(codexHome string) ([]Thread, string) {
	if threads, ok := discoverThreads(codexHome); ok {
		var out []Thread
		for _, t := range threads {
			if t.IsSubagent {
				continue
			}
			out = append(out, t)
		}
		return dedupeThreads(out), "sqlite"
	}

	files, err := globRollouts(codexHome)
	if err != nil {
		return nil, "glob"
	}
	var out []Thread
	for _, f := range files {
		t, skip := threadFromRollout(f)
		if skip {
			continue
		}
		out = append(out, t)
	}
	return dedupeThreads(out), "glob"
}

func dedupeThreads(threads []Thread) []Thread {
	seen := map[string]bool{}
	out := make([]Thread, 0, len(threads))
	for _, t := range threads {
		key := ""
		if t.RolloutPath != "" {
			key = "path:" + filepath.Clean(t.RolloutPath)
		} else if t.ID != "" {
			key = "id:" + t.ID
		}
		if key != "" {
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		out = append(out, t)
	}
	return out
}

// threadFromRollout reads just the session_meta of a rollout to build metadata
// for the glob fallback. The bool reports a subagent that should be skipped.
func threadFromRollout(path string) (Thread, bool) {
	f, err := os.Open(path)
	if err != nil {
		return Thread{}, true
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	t := Thread{RolloutPath: path}
	for sc.Scan() {
		raw := sc.Bytes()
		var line rolloutLine
		if json.Unmarshal(raw, &line) != nil {
			continue
		}
		if line.Type != "session_meta" {
			continue
		}
		if strings.Contains(string(line.Payload), "subagent") ||
			strings.Contains(string(line.Payload), "thread_spawn") {
			return Thread{}, true
		}
		var sm struct {
			ID  string `json:"id"`
			Cwd string `json:"cwd"`
		}
		_ = json.Unmarshal(line.Payload, &sm)
		t.ID = sm.ID
		t.Cwd = sm.Cwd
		break
	}
	return t, false
}
