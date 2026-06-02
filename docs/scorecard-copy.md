# 成绩卡文案表（zh / en 初版）

> 关联：[ADR 0009](adr/0009-i18n-scorecard-copy.md)（i18n 决策）、[ADR 0008](adr/0008-gamified-shareable-scorecard.md)（成绩卡设计）。
>
> 这是**预先写好的固定创意文案**——段位名、吐槽语、UI 标签。**人工本地化，非逐字翻译**：
> 英文是「另起一个地道的梗」，不是中文直译。称号 / 人格总结那段由模型按用户语言现写，**不**在此表内。
> 实现时把本表落成 skill 内的 i18n 资源（结构见 ADR 0009 OQ2）。
>
> 分寸（[ADR 0008](adr/0008-gamified-shareable-scorecard.md) D5）：只调侃**可改变的行为习惯**，不攻击能力 / 人格。
> 段位**从高到低**排列。

---

## 轴 1 · Prompt 功力 / Prompt Skill

> 信号：prompt 长度、结构化程度、约束清晰度、中途改需求频率。

| 段位（zh） | Tier (en) | 吐槽语（zh） | Roast (en) |
| --- | --- | --- | --- |
| 大师级 | Prompt Surgeon | 你的 prompt 像手术刀，一句话直达需求 | Your prompts are a scalpel — one line, straight to the point. |
| 老练 | Sharpshooter | 偶尔啰嗦，但基本一击即中 | A little wordy, but you usually land it first try. |
| 学徒 | Apprentice | 能用，就是话有点多 | Gets there — just a few too many words. |
| 复读机 | Broken Record | 同一个需求换三种说法说给 AI 听 | You pitch the same ask to the AI three different ways. |
| 玄学召唤师 | Vibe Summoner | 祈祷 AI 能读懂你的心 | You cast a vibe and pray the AI reads your mind. |

## 轴 2 · 烧钱姿势 / Spending Style

> 信号：花费 + 产出效率（值不值）。

| 段位（zh） | Tier (en) | 吐槽语（zh） | Roast (en) |
| --- | --- | --- | --- |
| 性价比刺客 | Value Assassin | 花得少干得多，刀刀见血 | Spends little, ships a lot — every token earns its keep. |
| 理性消费 | Sensible Spender | 该花花该省省，账单很健康 | Spends where it matters. Healthy bill. |
| 富哥随意 | High Roller | 成本？不存在的 | Cost? Never heard of it. |
| Opus 锤钉子 | Opus for a One-Liner | 拿最贵的模型干最简单的活 | Summoning Opus to fix a typo. |

## 轴 3 · 工程素养 / Engineering Sense

> 信号：session 收敛速度、是否善用 plan、改动是否聚焦、git 习惯。

| 段位（zh） | Tier (en) | 吐槽语（zh） | Roast (en) |
| --- | --- | --- | --- |
| 架构师 | Architect | plan 清晰，一气呵成 | Clear plan, clean execution. |
| 工程师 | Engineer | 稳扎稳打，该测的都测了 | Steady hands — tests where they matter. |
| 莽夫 | Cowboy Coder | 不写 plan，直接梭哈 | No plan. Just send it. |
| 考古学家 | Archaeologist | 一个 session 跨三个无关项目，git diff 看不懂自己干了啥 | One session, three unrelated projects — you can't read your own git diff. |

## 轴 4 · 勤奋度 / Diligence（纯娱乐）

> 信号：日活、连续天数、深夜使用占比。

| 段位（zh） | Tier (en) | 吐槽语（zh） | Roast (en) |
| --- | --- | --- | --- |
| 劳模 | Workhorse | 天天打卡，AI 都怕你 | Clocks in daily. Even the AI needs a break. |
| 996 战士 | Crunch Lord | 早 9 晚 9，深夜还在改 bug | 9-to-9, still fixing bugs at midnight. |
| 养生程序员 | Zen Coder | 准点下班，周末不碰键盘 | Logs off on time. Weekends are sacred. |
| 周末才想起来 | Weekend Warrior | 周一到周五人间蒸发 | Vanishes Monday through Friday. |

---

## UI 标签 / UI labels

| key | zh | en |
| --- | --- | --- |
| `report_title` | AI 用量教练报告 | AI Usage Coach Report |
| `scorecard` | 成绩卡 | Scorecard |
| `tier` | 段位 | Tier |
| `title_label` | 称号 | Title |
| `beats_pct` | 超过了 {pct}% 的用户 | Beats {pct}% of users |
| `axis_prompt` | Prompt 功力 | Prompt Skill |
| `axis_spending` | 烧钱姿势 | Spending Style |
| `axis_engineering` | 工程素养 | Engineering Sense |
| `axis_diligence` | 勤奋度 | Diligence |
| `local_privacy_note` | 全部分析在你本地完成，prompt 内容不离开你的机器 | Analyzed entirely on your machine — your prompts never leave it. |
| `estimate_note` | 排名为本地估算，仅供娱乐 | Ranking is a local estimate, just for fun. |

> 注：`{pct}` 等占位符由渲染层填值；称号 / 人格总结整段由模型按用户语言生成，不在本表。
</content>
