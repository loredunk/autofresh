package codexreport

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// Options configures a usage report run. Date/Since/Days select the window
// (mutually combined as documented on the CLI); when all are empty the window is
// "today" in the machine's local timezone.
type Options struct {
	Date   string // YYYY-MM-DD, a single local day
	Since  string // YYYY-MM-DD, from this day through end of today
	Days   int    // last N days including today
	JSON   bool
	ByRepo bool

	// Now and Loc are injectable for tests; zero values mean "real now / Local".
	Now time.Time
	Loc *time.Location
}

// Report is the fully-aggregated result, also the JSON output shape.
type Report struct {
	GeneratedFor string `json:"generated_for"` // window description
	Timezone     string `json:"timezone"`
	Source       string `json:"source"` // "sqlite" or "glob"
	CodexHome    string `json:"codex_home"`

	Sessions        int    `json:"sessions"`
	DurationSeconds int64  `json:"duration_seconds"`
	Duration        string `json:"duration"`

	Tokens struct {
		Input           int64 `json:"input"`
		CachedInput     int64 `json:"cached_input"`
		Output          int64 `json:"output"`
		ReasoningOutput int64 `json:"reasoning_output"`
		Total           int64 `json:"total"`
	} `json:"tokens"`

	CacheHitRate   float64 `json:"cache_hit_rate"`
	ReasoningRatio float64 `json:"reasoning_ratio"`

	EstimatedCostUSD float64  `json:"estimated_cost_usd"`
	Models           []string `json:"models"`
	UnpricedModels   []string `json:"unpriced_models,omitempty"`

	Tools struct {
		ShellCalls  int            `json:"shell_calls"`
		WebSearches int            `json:"web_searches"`
		FileChanges int            `json:"file_changes"`
		TotalCalls  int            `json:"total_calls"`
		TopCommands []CommandCount `json:"top_commands"`
	} `json:"tools"`

	Repos     []RepoReport      `json:"repos"`
	Hours     []HourReport      `json:"hours"`
	Sources   []UsageReport     `json:"sources"`
	Languages []UsageReport     `json:"languages"`
	Git       GitHabitsReport   `json:"git_habits"`
	Project   ProjectMgmtReport `json:"project_management"`
	Codex     CodexConfigReport `json:"codex"`

	// Glossary explains each metric's meaning and caveats so an agent can
	// interpret the JSON without extra context. It is emitted in JSON output
	// only (the text renderer ignores it).
	Glossary map[string]string `json:"glossary,omitempty"`
}

// reportGlossary is the self-describing field reference for `--json` consumers.
// Keep keys aligned with the JSON field names above.
var reportGlossary = map[string]string{
	"_about":             "仅本机数据，不跨机器汇总；不含任何账户级配额百分比（CLI 下 rate_limits 恒为 null）。",
	"cache_hit_rate":     "cached_input / input，缓存命中率；越高越省钱（重复上下文被缓存复用）。",
	"reasoning_ratio":    "reasoning_output / output，推理 token 占输出的比例；偏高常意味任务被反复推理。",
	"estimated_cost_usd": "估算成本 = token × 内置参考价，仅供参考，不等于实际账单。",
	"tokens":             "input/cached_input/output/reasoning_output/total，按会话求增量去重后的累计值。",
	"sources":            "用量来源拆分（CLI / Codex App / IDE 插件等，若可识别）。",
	"git_habits":         "git 子命令频次与评审/风险信号（如只 diff/status 不 commit）。",
	"project_management": "各仓库是否有测试/构建/CI，以及文档/配置改动信号。",
	"duration":           "活跃时长（相邻事件间隔 ≤5 分钟才计入），非墙钟跨度。",
}

type CommandCount struct {
	Command string `json:"command"`
	Count   int    `json:"count"`
}

type RepoReport struct {
	Repo            string            `json:"repo"`
	Branches        []string          `json:"branches,omitempty"`
	Sessions        int               `json:"sessions"`
	Tokens          int64             `json:"tokens"`
	CostUSD         float64           `json:"estimated_cost_usd"`
	Language        string            `json:"language,omitempty"`
	LanguageMix     []LanguageCount   `json:"language_mix,omitempty"`
	BuildSystems    []string          `json:"build_systems,omitempty"`
	TestCommands    []string          `json:"test_commands,omitempty"`
	FileChangeTypes []FileChangeCount `json:"file_change_types,omitempty"`
}

type HourReport struct {
	Hour   int   `json:"hour"`
	Tokens int64 `json:"tokens"`
}

type UsageReport struct {
	Name     string `json:"name"`
	Sessions int    `json:"sessions"`
	Tokens   int64  `json:"tokens"`
}

type LanguageCount struct {
	Language string `json:"language"`
	Files    int    `json:"files"`
}

type FileChangeCount struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

type CodexConfigReport struct {
	UserConfig              ConfigSummary        `json:"user_config"`
	ProfileConfigs          []ConfigSummary      `json:"profile_configs,omitempty"`
	GlobalInstructionFiles  []InstructionSummary `json:"global_instruction_files,omitempty"`
	ProjectInstructionFiles int                  `json:"project_instruction_files"`
	HistoryPersistence      string               `json:"history_persistence,omitempty"`
}

type ConfigSummary struct {
	Path    string            `json:"path"`
	Exists  bool              `json:"exists"`
	Keys    map[string]string `json:"keys,omitempty"`
	Secrets []string          `json:"secrets,omitempty"`
}

type InstructionSummary struct {
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

type GitHabitsReport struct {
	CommandCount     int            `json:"command_count"`
	TopSubcommands   []CommandCount `json:"top_subcommands,omitempty"`
	BranchCount      int            `json:"branch_count"`
	MultiBranchRepos int            `json:"multi_branch_repos"`
	ReviewSignals    []string       `json:"review_signals,omitempty"`
	RiskSignals      []string       `json:"risk_signals,omitempty"`
}

type ProjectMgmtReport struct {
	ReposWithTests       int      `json:"repos_with_tests"`
	ReposWithBuildSystem int      `json:"repos_with_build_system"`
	ReposWithCI          int      `json:"repos_with_ci"`
	PlanningFileChanges  int      `json:"planning_file_changes"`
	DocumentationChanges int      `json:"documentation_changes"`
	ConfigChanges        int      `json:"config_changes"`
	Signals              []string `json:"signals,omitempty"`
}

// Run generates and renders a report to out.
func Run(opts Options, out io.Writer) error {
	rep, err := Build(opts)
	if err != nil {
		return err
	}
	if opts.JSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}
	return renderText(rep, opts.ByRepo, out)
}

// Build assembles the Report without rendering, so it can be tested directly.
func Build(opts Options) (Report, error) {
	loc := opts.Loc
	if loc == nil {
		loc = time.Local
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	now = now.In(loc)

	from, to, desc, err := resolveWindow(opts, now, loc)
	if err != nil {
		return Report{}, err
	}

	home, err := CodexHome()
	if err != nil {
		return Report{}, err
	}

	threads, source := loadThreadsForRange(home)
	agg := newAggregate()
	po := parseOptions{from: from, to: to, loc: loc}
	for _, t := range threads {
		parseThread(t, po, agg)
	}

	return assemble(agg, desc, source, home, loc), nil
}

func resolveWindow(opts Options, now time.Time, loc *time.Location) (from, to time.Time, desc string, err error) {
	startOfDay := func(t time.Time) time.Time {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	}
	today := startOfDay(now)
	tomorrow := today.AddDate(0, 0, 1)

	switch {
	case opts.Date != "":
		d, e := time.ParseInLocation("2006-01-02", opts.Date, loc)
		if e != nil {
			return from, to, "", fmt.Errorf("invalid --date %q (want YYYY-MM-DD)", opts.Date)
		}
		return d, d.AddDate(0, 0, 1), opts.Date, nil
	case opts.Since != "":
		d, e := time.ParseInLocation("2006-01-02", opts.Since, loc)
		if e != nil {
			return from, to, "", fmt.Errorf("invalid --since %q (want YYYY-MM-DD)", opts.Since)
		}
		return d, tomorrow, fmt.Sprintf("%s 至 %s", opts.Since, today.Format("2006-01-02")), nil
	case opts.Days > 0:
		start := today.AddDate(0, 0, -(opts.Days - 1))
		return start, tomorrow, fmt.Sprintf("最近 %d 天 (%s 至 %s)", opts.Days, start.Format("2006-01-02"), today.Format("2006-01-02")), nil
	default:
		return today, tomorrow, today.Format("2006-01-02"), nil
	}
}

func assemble(agg *aggregate, desc, source, home string, loc *time.Location) Report {
	var r Report
	r.GeneratedFor = desc
	zoneName, offset := time.Now().In(loc).Zone()
	r.Timezone = fmt.Sprintf("%s (UTC%+d)", zoneName, offset/3600)
	r.Source = source
	r.CodexHome = home
	r.Glossary = reportGlossary

	r.Sessions = len(agg.sessionIDs)
	r.DurationSeconds = int64(agg.duration.Seconds())
	r.Duration = humanizeDuration(agg.duration)

	r.Tokens.Input = agg.tokens.Input
	r.Tokens.CachedInput = agg.tokens.CachedInput
	r.Tokens.Output = agg.tokens.Output
	r.Tokens.ReasoningOutput = agg.tokens.ReasoningOutput
	r.Tokens.Total = agg.tokens.Total

	if agg.tokens.Input > 0 {
		r.CacheHitRate = float64(agg.tokens.CachedInput) / float64(agg.tokens.Input)
	}
	if agg.tokens.Output > 0 {
		r.ReasoningRatio = float64(agg.tokens.ReasoningOutput) / float64(agg.tokens.Output)
	}

	r.EstimatedCostUSD = agg.cost
	r.Models = sortedKeys(agg.modelsSeen)
	r.UnpricedModels = sortedKeys(agg.missingPrice)

	r.Tools.ShellCalls = agg.shellCalls
	r.Tools.WebSearches = agg.webSearches
	r.Tools.FileChanges = agg.fileChanges
	r.Tools.TotalCalls = agg.totalCalls
	r.Tools.TopCommands = topCommands(agg.shellCommands, 12)

	for _, ra := range agg.byRepo {
		r.Repos = append(r.Repos, RepoReport{
			Repo:            ra.repo,
			Branches:        sortedKeys(ra.branches),
			Sessions:        len(ra.sessions),
			Tokens:          ra.tokens.Total,
			CostUSD:         ra.cost,
			Language:        ra.language,
			LanguageMix:     ra.languageMix,
			BuildSystems:    sortedKeys(ra.buildSystems),
			TestCommands:    sortedKeys(ra.testCommands),
			FileChangeTypes: topFileChangeTypes(ra.fileTypes, 8),
		})
	}
	sort.Slice(r.Repos, func(i, j int) bool { return r.Repos[i].Tokens > r.Repos[j].Tokens })

	for h := 0; h < 24; h++ {
		if agg.byHour[h].Total > 0 {
			r.Hours = append(r.Hours, HourReport{Hour: h, Tokens: agg.byHour[h].Total})
		}
	}
	r.Sources = usageReports(agg.bySource)
	r.Languages = usageReports(agg.byLanguage)
	r.Git = buildGitHabits(agg, r.Repos)
	r.Project = buildProjectMgmt(r.Repos)
	r.Codex = scanCodexConfig(home, agg)
	return r
}

func usageReports(groups map[string]*usageAgg) []UsageReport {
	out := make([]UsageReport, 0, len(groups))
	for _, g := range groups {
		out = append(out, UsageReport{
			Name:     g.name,
			Sessions: len(g.sessions),
			Tokens:   g.tokens.Total,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Tokens != out[j].Tokens {
			return out[i].Tokens > out[j].Tokens
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func topFileChangeTypes(m map[string]int, n int) []FileChangeCount {
	out := make([]FileChangeCount, 0, len(m))
	for typ, count := range m {
		if count > 0 {
			out = append(out, FileChangeCount{Type: typ, Count: count})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Type < out[j].Type
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

func topCommands(m map[string]int, n int) []CommandCount {
	var cc []CommandCount
	for k, v := range m {
		cc = append(cc, CommandCount{Command: k, Count: v})
	}
	sort.Slice(cc, func(i, j int) bool {
		if cc[i].Count != cc[j].Count {
			return cc[i].Count > cc[j].Count
		}
		return cc[i].Command < cc[j].Command
	})
	if len(cc) > n {
		cc = cc[:n]
	}
	return cc
}

func sortedKeys(m map[string]bool) []string {
	var out []string
	for k := range m {
		if k != "" {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func humanizeDuration(d time.Duration) string {
	if d <= 0 {
		return "0m"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func renderText(r Report, byRepo bool, out io.Writer) error {
	var b strings.Builder

	fmt.Fprintf(&b, "Codex 使用报告 · %s · %s\n", r.GeneratedFor, r.Timezone)
	fmt.Fprintf(&b, "本机 %s · 仅本机数据 (来源: %s) · %d 个会话 · 时长 %s\n\n",
		r.CodexHome, r.Source, r.Sessions, r.Duration)

	if r.Tokens.Total == 0 {
		fmt.Fprintln(&b, "（该时间窗口内没有 Codex 使用记录）")
		_, err := io.WriteString(out, b.String())
		return err
	}

	fmt.Fprintln(&b, "Token")
	fmt.Fprintf(&b, "  input %s · cached %s · output %s · reasoning %s · total %s\n",
		comma(r.Tokens.Input), comma(r.Tokens.CachedInput), comma(r.Tokens.Output),
		comma(r.Tokens.ReasoningOutput), comma(r.Tokens.Total))
	fmt.Fprintf(&b, "  缓存命中率 %.1f%% · reasoning 占 output %.1f%%\n",
		r.CacheHitRate*100, r.ReasoningRatio*100)

	modelNote := ""
	if len(r.Models) > 0 {
		modelNote = " · 模型: " + strings.Join(r.Models, ", ")
	}
	fmt.Fprintf(&b, "  估算成本 $%.2f（估算价，仅供参考%s）\n", r.EstimatedCostUSD, modelNote)
	if len(r.UnpricedModels) > 0 {
		fmt.Fprintf(&b, "  注意: 以下模型无内置价格，未计入成本: %s\n", strings.Join(r.UnpricedModels, ", "))
	}
	fmt.Fprintln(&b)

	fmt.Fprintf(&b, "工具调用 (共 %d)\n", r.Tools.TotalCalls)
	fmt.Fprintf(&b, "  shell %d · web 搜索 %d · 改文件 %d\n",
		r.Tools.ShellCalls, r.Tools.WebSearches, r.Tools.FileChanges)
	if len(r.Tools.TopCommands) > 0 {
		parts := make([]string, 0, len(r.Tools.TopCommands))
		for _, c := range r.Tools.TopCommands {
			parts = append(parts, fmt.Sprintf("%s(%d)", c.Command, c.Count))
		}
		fmt.Fprintf(&b, "  top 命令: %s\n", strings.Join(parts, " "))
	}
	fmt.Fprintln(&b)

	if len(r.Sources) > 0 {
		fmt.Fprintln(&b, "按来源")
		renderUsageBreakdown(&b, r.Sources, r.Tokens.Total)
		fmt.Fprintln(&b)
	}

	if len(r.Languages) > 0 {
		fmt.Fprintln(&b, "按语言（根据本机仓库文件估算）")
		renderUsageBreakdown(&b, r.Languages, r.Tokens.Total)
		fmt.Fprintln(&b)
	}

	fmt.Fprintln(&b, "习惯")
	fmt.Fprintf(&b, "  Git 命令 %d 次 · 分支上下文 %d 个 · 多分支仓库 %d 个\n",
		r.Git.CommandCount, r.Git.BranchCount, r.Git.MultiBranchRepos)
	if len(r.Project.Signals) > 0 {
		fmt.Fprintf(&b, "  项目管理: %s\n", strings.Join(r.Project.Signals, "；"))
	}
	fmt.Fprintln(&b)

	if len(r.Repos) > 0 {
		fmt.Fprintln(&b, "按仓库")
		limit := len(r.Repos)
		if !byRepo && limit > 8 {
			limit = 8
		}
		for _, rr := range r.Repos[:limit] {
			branch := ""
			if byRepo && len(rr.Branches) > 0 {
				branch = " [" + strings.Join(rr.Branches, ",") + "]"
			}
			detail := repoDetail(rr)
			if detail != "" {
				detail = " · " + detail
			}
			fmt.Fprintf(&b, "  %-24s %d 会话  %s token  $%.2f%s%s\n",
				truncate(rr.Repo, 24), rr.Sessions, comma(rr.Tokens), rr.CostUSD, branch, detail)
		}
		if !byRepo && len(r.Repos) > limit {
			fmt.Fprintf(&b, "  …另有 %d 个仓库（用 --by-repo 查看全部）\n", len(r.Repos)-limit)
		}
		fmt.Fprintln(&b)
	}

	if len(r.Hours) > 0 {
		fmt.Fprintln(&b, "按时段 (本机时间)")
		renderHours(&b, r.Hours, r.Tokens.Total)
	}

	_, err := io.WriteString(out, b.String())
	return err
}

func renderUsageBreakdown(b *strings.Builder, rows []UsageReport, total int64) {
	limit := len(rows)
	if limit > 8 {
		limit = 8
	}
	for _, row := range rows[:limit] {
		pct := 0.0
		if total > 0 {
			pct = float64(row.Tokens) / float64(total) * 100
		}
		fmt.Fprintf(b, "  %-16s %d 会话  %5.1f%%  %s token\n",
			truncate(row.Name, 16), row.Sessions, pct, comma(row.Tokens))
	}
	if len(rows) > limit {
		fmt.Fprintf(b, "  …另有 %d 项\n", len(rows)-limit)
	}
}

func repoDetail(rr RepoReport) string {
	var parts []string
	if rr.Language != "" {
		parts = append(parts, rr.Language)
	}
	if len(rr.BuildSystems) > 0 {
		parts = append(parts, strings.Join(limitStrings(rr.BuildSystems, 2), "+"))
	}
	if len(rr.TestCommands) > 0 {
		parts = append(parts, "测试:"+strings.Join(limitStrings(rr.TestCommands, 2), ", "))
	}
	if len(rr.FileChangeTypes) > 0 {
		var changed []string
		for _, fc := range rr.FileChangeTypes {
			changed = append(changed, fmt.Sprintf("%s(%d)", fc.Type, fc.Count))
			if len(changed) == 2 {
				break
			}
		}
		parts = append(parts, "变更:"+strings.Join(changed, ", "))
	}
	return strings.Join(parts, " · ")
}

func limitStrings(values []string, n int) []string {
	if len(values) <= n {
		return values
	}
	return values[:n]
}

func renderHours(b *strings.Builder, hours []HourReport, total int64) {
	var max int64
	for _, h := range hours {
		if h.Tokens > max {
			max = h.Tokens
		}
	}
	for _, h := range hours {
		bars := 0
		if max > 0 {
			bars = int(float64(h.Tokens) / float64(max) * 20)
		}
		pct := 0.0
		if total > 0 {
			pct = float64(h.Tokens) / float64(total) * 100
		}
		fmt.Fprintf(b, "  %02d:00  %-20s %5.1f%%  %s\n",
			h.Hour, strings.Repeat("█", bars), pct, comma(h.Tokens))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// comma formats an integer with thousands separators.
func comma(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	out := strings.Join(parts, ",")
	if neg {
		return "-" + out
	}
	return out
}
