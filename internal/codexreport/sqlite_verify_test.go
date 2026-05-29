package codexreport

import (
	"fmt"
	"testing"
)

// TestReadThreadsAgainstRealDB is a manual smoke test against the developer's
// own CODEX_HOME. It is skipped automatically when no state DB is present.
func TestReadThreadsAgainstRealDB(t *testing.T) {
	home, err := CodexHome()
	if err != nil {
		t.Skip(err)
	}
	threads, ok := discoverThreads(home)
	if !ok {
		t.Skip("no usable state sqlite")
	}
	fmt.Printf("threads=%d\n", len(threads))
	sub := 0
	for _, th := range threads {
		if th.IsSubagent {
			sub++
		}
	}
	fmt.Printf("subagents=%d (excluded at report time)\n", sub)
	for i, th := range threads {
		if i >= 5 {
			break
		}
		fmt.Printf("  id=%.8s created=%d branch=%q model=%q sub=%v path=...%.30s\n",
			th.ID, th.CreatedAt, th.GitBranch, th.Model, th.IsSubagent, tail(th.RolloutPath, 30))
	}
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
