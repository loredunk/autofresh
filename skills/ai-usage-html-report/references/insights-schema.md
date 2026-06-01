# Insights JSON Schema

Create a JSON file with this shape:

```json
{
  "title": "Codex 使用深度报告",
  "subtitle": "2026-06-01 本机使用复盘",
  "executive_summary": [
    "短句总结一条重要发现",
    "短句总结另一条重要发现"
  ],
  "recommendations": [
    {
      "title": "建议标题",
      "priority": "high|medium|low",
      "evidence": "来自 report JSON 的证据",
      "action": "具体下一步"
    }
  ],
  "insight_ladder": [
    {
      "title": "长会话与大上下文复用",
      "evidence": [
        "total tokens 约 2549 万",
        "cached_input 占比约 96%",
        "autofresh 2 个 session 占当天绝大多数 token"
      ],
      "meaning": "这些指标说明什么使用模式",
      "impact": "它为什么影响成本、速度、质量或工作方式",
      "drilldown": "下一步应该看哪个项目、session、时间段或工具循环",
      "intervention": "应该如何拆 session、调整 prompt、写 AGENTS.md 或沉淀 skill"
    }
  ],
  "sections": [
    {
      "title": "Git 使用习惯",
      "bullets": [
        "观察到的习惯或风险",
        "可执行改进"
      ]
    },
    {
      "title": "项目管理习惯",
      "bullets": [
        "测试、CI、文档、计划、配置方面的洞察"
      ]
    }
  ],
  "project_notes": [
    {
      "repo": "repo-name",
      "summary": "该项目当天的使用画像",
      "next_action": "建议下一步"
    }
  ],
  "session_reviews": [
    {
      "repo": "repo-name",
      "session_id": "optional-id",
      "rollout_path": "/path/to/rollout.jsonl",
      "summary": "该 session 的 prompt / token 使用诊断",
      "token_drivers": [
        "可能拉高 token 的原因"
      ],
      "prompt_issues": [
        "prompt 模糊、过窄、缺少验收方式等问题"
      ],
      "better_first_prompt": "更好的起始 prompt",
      "better_followup_prompt": "更好的追问 prompt",
      "next_action": "沉淀到 AGENTS.md、skill、检查清单或项目文档的动作"
    }
  ]
}
```

Rules:

- Use Chinese unless the user asks otherwise.
- Keep each bullet concise.
- Mention uncertainty when evidence is incomplete.
- Use facts from `report --json`; do not invent account-level billing or server-side usage.
- Include `insight_ladder` when the data supports deeper interpretations. Each item should connect evidence to meaning, impact, drilldown, and intervention.
- Only include `session_reviews` when the user selected a specific session and approved reading user prompts for that session.
- Prefer prompt rewrites and paraphrased diagnoses over verbatim prompt quotes.
