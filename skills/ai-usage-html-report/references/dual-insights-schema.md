# Dual-Platform Insights JSON Schema

This is the insights file consumed by `scripts/render_dual_platform.py` (the
**dual-platform** Claude Code + Codex report). It is different from the
Codex-only `insights-schema.md`: the dual renderer already builds the
comparison cards, model tables, token-composition bars, and active-hours chart
from the merged `/tmp/ai-usage.json` data, so the insights file only carries the
**AI interpretation** layer.

Write `/tmp/ai-usage-insights.json` with this shape:

```json
{
  "title": "双平台 AI 使用报告",
  "executive_summary": "一到两段话总结两平台的整体使用情况、成本/token 分布、最值得注意的发现。可用换行分段。",
  "recommendations": [
    "一条纯文本建议",
    {
      "title": "建议标题",
      "text": "具体下一步动作",
      "evidence": "来自合并数据的证据，例如 Claude Code 缓存命中率 92% 而 Codex 仅 40%"
    }
  ],
  "insights": [
    "一条纯文本洞见",
    {
      "title": "长会话与大上下文复用",
      "detail": "Codex 单日 token 中 cached_input 占 96%，说明会话很长、上下文反复重放。"
    }
  ]
}
```

## 字段说明

- `title`（可选，字符串）：报告标题。缺省时渲染为「双平台 AI 使用报告」。
- `executive_summary`（可选）：渲染在报告靠前的醒目「执行摘要」区。
  - 可以是**字符串**（用 `\n` 分段，每段一个 `<p>`），或**字符串列表**（渲染成无序列表）。
- `recommendations`（可选，列表）：渲染成「AI 建议」卡片。每项可以是：
  - **字符串**：直接作为建议正文。
  - **对象**：`{title?, text|action, evidence?}`。`title` 作为卡片标题（加粗），
    `text`（或兼容旧字段 `action`）为正文，`evidence` 存在时显示为「证据：…」灰字。
- `insights`（可选，列表）：渲染成「AI 洞见」列表。每项可以是：
  - **字符串**：直接作为一条洞见。
  - **对象**：`{title?, detail}`。有 `title` 时渲染为「标题：detail」，否则只渲染 `detail`。

## 兼容性

- **所有字段都可缺省**，renderer 不会因缺字段报错。
- 只提供 `{"insights": ["...", "..."]}`（旧的扁平结构）仍能正常渲染。
- 同一列表里可以混用字符串项和对象项。

## 写作规则

- 默认中文，除非用户要求其它语言。
- 数字必须来自 `/tmp/ai-usage.json` 的真实合并结果（两平台都要覆盖）。
- 推断性结论要表述为推断，并在证据不全时说明不确定性。
- 不要编造账号级账单或服务端用量；这是本机证据。
- 不要嵌入任何 prompt 原文、指令文件内容或密钥。
- 可参考 `insight-patterns.md` 的「证据 → 含义 → 影响 → 深入 → 干预」阶梯，
  把分析浓缩进 `recommendations` 的 `evidence`/`text` 和 `insights` 的 `detail` 里。
