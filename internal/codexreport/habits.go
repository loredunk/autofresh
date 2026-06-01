package codexreport

import (
	"fmt"
	"strings"
)

func buildGitHabits(agg *aggregate, repos []RepoReport) GitHabitsReport {
	var r GitHabitsReport
	for _, count := range agg.gitCommands {
		r.CommandCount += count
	}
	r.TopSubcommands = topCommands(agg.gitCommands, 10)

	branches := map[string]bool{}
	for _, repo := range repos {
		for _, branch := range repo.Branches {
			branches[repo.Repo+"@"+branch] = true
		}
		if len(repo.Branches) > 1 {
			r.MultiBranchRepos++
		}
	}
	r.BranchCount = len(branches)

	if agg.gitCommands["status"] > 0 {
		r.ReviewSignals = append(r.ReviewSignals, fmt.Sprintf("经常检查工作区状态: git status %d 次", agg.gitCommands["status"]))
	}
	if agg.gitCommands["diff"] > 0 {
		r.ReviewSignals = append(r.ReviewSignals, fmt.Sprintf("会查看差异: git diff %d 次", agg.gitCommands["diff"]))
	}
	if agg.gitCommands["log"] > 0 || agg.gitCommands["show"] > 0 {
		r.ReviewSignals = append(r.ReviewSignals, "会读取历史上下文")
	}
	if agg.gitCommands["commit"] == 0 && (agg.gitCommands["diff"] > 0 || agg.gitCommands["status"] > 0) {
		r.RiskSignals = append(r.RiskSignals, "观察到检查状态/差异，但没有提交命令；可能偏向让人类最后提交")
	}
	if agg.gitCommands["push"] > 0 {
		r.RiskSignals = append(r.RiskSignals, "观察到 push 命令；适合在 AGENTS.md 中写清推送前检查")
	}
	return r
}

func buildProjectMgmt(repos []RepoReport) ProjectMgmtReport {
	var r ProjectMgmtReport
	for _, repo := range repos {
		if len(repo.TestCommands) > 0 {
			r.ReposWithTests++
		}
		if len(repo.BuildSystems) > 0 {
			r.ReposWithBuildSystem++
		}
		if hasBuildSystem(repo.BuildSystems, "GitHub Actions") {
			r.ReposWithCI++
		}
		for _, fc := range repo.FileChangeTypes {
			switch fc.Type {
			case "Markdown":
				r.DocumentationChanges += fc.Count
			case "Project config", "YAML", "TOML", "JSON":
				r.ConfigChanges += fc.Count
			}
		}
	}
	r.PlanningFileChanges = r.DocumentationChanges

	if len(repos) > 0 {
		if r.ReposWithTests == 0 {
			r.Signals = append(r.Signals, "活跃项目中没有观察到测试命令")
		} else {
			r.Signals = append(r.Signals, fmt.Sprintf("%d/%d 个活跃项目观察到测试命令", r.ReposWithTests, len(repos)))
		}
		if r.ReposWithCI > 0 {
			r.Signals = append(r.Signals, fmt.Sprintf("%d 个活跃项目检测到 GitHub Actions", r.ReposWithCI))
		}
		if r.DocumentationChanges > 0 {
			r.Signals = append(r.Signals, fmt.Sprintf("文档/计划类变更 %d 次", r.DocumentationChanges))
		}
		if r.ConfigChanges > 0 {
			r.Signals = append(r.Signals, fmt.Sprintf("配置类变更 %d 次", r.ConfigChanges))
		}
	}
	return r
}

func hasBuildSystem(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(value, want) {
			return true
		}
	}
	return false
}
