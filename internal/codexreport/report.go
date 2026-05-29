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

	Repos []RepoReport `json:"repos"`
	Hours []HourReport `json:"hours"`
}

type CommandCount struct {
	Command string `json:"command"`
	Count   int    `json:"count"`
}

type RepoReport struct {
	Repo     string   `json:"repo"`
	Branches []string `json:"branches,omitempty"`
	Sessions int      `json:"sessions"`
	Tokens   int64    `json:"tokens"`
	CostUSD  float64  `json:"estimated_cost_usd"`
}

type HourReport struct {
	Hour   int   `json:"hour"`
	Tokens int64 `json:"tokens"`
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
			Repo:     ra.repo,
			Branches: sortedKeys(ra.branches),
			Sessions: len(ra.sessions),
			Tokens:   ra.tokens.Total,
			CostUSD:  ra.cost,
		})
	}
	sort.Slice(r.Repos, func(i, j int) bool { return r.Repos[i].Tokens > r.Repos[j].Tokens })

	for h := 0; h < 24; h++ {
		if agg.byHour[h].Total > 0 {
			r.Hours = append(r.Hours, HourReport{Hour: h, Tokens: agg.byHour[h].Total})
		}
	}
	return r
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
			fmt.Fprintf(&b, "  %-24s %d 会话  %s token  $%.2f%s\n",
				truncate(rr.Repo, 24), rr.Sessions, comma(rr.Tokens), rr.CostUSD, branch)
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
