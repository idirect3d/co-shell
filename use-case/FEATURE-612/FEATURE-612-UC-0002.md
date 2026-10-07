# FEATURE-612 UC-0002：环境块显示优化（公告板名册 / 可用工具仅路径）

> 分支：**FEATURE-612**　版本：**v0.100.1**　BUILD：**1243**
> 现象：动态信息面板环境块区里，`<board_agents>` 标题为「公告板代理」，内容按通用规则渲染——块内每个 `<agent>` 走「标签：值」规则，显示的是职责正文；名册无变化时该块被后端精简为 `<unchanged/>`，面板只显示「（与上次相同，无变化）」，用户看不出「谁是 online」。`<tools>` 每行由后端附带 `日期 时间 大小`，路径被淹没。
> 判定口径：① `board_agents` 块标题显示为**「公告板」**，块内**一行一个 agent**（agent 名称 + online/offline **彩色小徽章**），不再显示 id、职责、rev 等多余信息；块为 `<unchanged/>` 精简态时**沿用上次解析到的名册**。② `tools` 块每行**只显示工具路径**，不显示日期/时间/大小。

## 背景（现状与缺口）

- 现状：`web/static/app.js` 的 `dynEnvLines(tag, raw)` 对所有块走同一套解析——子元素渲染成 `标签：值`，无子元素的块按行保留原文；`dynBlockEl(b)` 把行文本直接塞进 `.dyn-env-line`。
- 现状：块标题由 `dynEnvLabels` 决定，`board_agents` 的中文标签是「公告板代理」。
- 现状：后端为省 token，`<board_agents>` 在名册版本（rev）不变时只下发 `<unchanged/>`（`agent/env_slim.go` 的 `boardRosterBlockSlim`）；`<tools>` 行由目录列举生成，形如 `bin/x.py 2026-07-30 05:43:15 1.4KB`。
- 缺口：面板看不出公告板成员与在线状态；工具列表的可读性被时间/大小列稀释。
- 本次：仅前端展示层改造（`app.js` + `style.css`），后端与线协议不变。

## 变更点（实际）

| 位置 | 变更 |
|---|---|
| `web/static/app.js`（i18n） | `dynEnvLabels.board_agents`：中文「公告板代理」→「公告板」；英文「Board agents」→「Board」 |
| `web/static/app.js`（新增 `dynEnvPath`） | 剥掉目录列举行尾部的 `日期 时间 [大小]`，只留路径 |
| `web/static/app.js`（新增 `dynBoardAgents`） | 解析 `<agent id=… name=… status=…>`（含自闭合形式），输出 `{text: 名称, badge: online/offline}` 行；有明细时写入模块级缓存 `lastBoardAgents`，`<unchanged/>` 时直接返回缓存副本 |
| `web/static/app.js`（`dynEnvLines`） | `board_agents` 特判走 `dynBoardAgents`；无子元素块按行渲染时 `tools` 走 `dynEnvPath`，其余块保持 `dynEnvValue` 原逻辑 |
| `web/static/app.js`（`dynBlockEl`） | 行既可以是字符串，也可以是 `{text, badge}` 对象；后者追加一枚 `.dyn-env-badge` 徽章元素 |
| `web/static/app.js`（`session_switched`） | 会话切换时清空 `lastBoardAgents`，避免把上一会话的名册带到新会话 |
| `web/static/style.css` | 新增 `.dyn-env-badge`（胶囊底、小字号）及 `.on`（绿）/`.off`（灰）配色 |

## 用例与结果

> `(R)` = 可自动化（语法检查 / 浏览器脚本注入复现），`(M)` = 需人工在 Web UI 验收。结果列填写本次实测结论。

### A. 公告板块

- **UC-01（M）** 块标题：环境块区中该块标题显示为「公告板」。
- **UC-02（M）** 明细态逐行：块内每个 agent 占一行，行内容为「agent 名称」；名册有 2 个 agent 时显示 2 行。
- **UC-03（M）** 状态徽章：名称右侧跟 `online`（绿色胶囊）/ `offline`（灰色胶囊）徽章，同屏可区分。
- **UC-04（M）** 无多余信息：行内不出现 agent id、职责文本、`rev` 值或 XML 角标。
- **UC-05（M）** 精简态沿用：后端下发 `<board_agents rev="N"><unchanged/></board_agents>` 时，面板仍显示上一次解析到的同一批「名称 + 状态」行，而不是「（与上次相同，无变化）」。
- **UC-06（M）** 会话隔离：切换到另一个会话后，公告板区块不显示上一会话的名册（缓存已清空，按新会话内容渲染）。
- **UC-07（M）** 自闭合 agent：形如 `<agent id="a" name="Ops" status="offline"/>` 的自闭合节点同样能解析成一行。
- **UC-08（M）** 空名册兜底：块内既无 agent 也无缓存时显示「无」，不报错、不塌陷。

### B. 可用工具块

- **UC-09（M）** 仅路径：每行只显示 `bin/…` 路径，行内不出现日期、时间与文件大小。
- **UC-10（M）** 无大小文件：源文件本就无扩展名/无大小列（如 `bin/compare_xlsx`）时，路径完整保留、不被截断。
- **UC-11（M）** 排序与位置：`<tools>` 仍排在环境块区最下方，行序与后端下发顺序一致。

### C. 回归

- **UC-12（M）** 其他块不变：`<runtime_info>`、`<context_window>`、`<user_presence>`、`<opened_resources>` 等块的渲染结果与改动前一致（仍为「标签：值」）。
- **UC-13（M）** 英文界面：语言切到 English 时该块标题为「Board」，行为一致。
- **UC-14（R）** 语法与编译：`node --check web/static/app.js` 通过；`go build ./... && go vet ./...` 全绿。

## 实测记录（BUILD-1243）

- UC-01 / UC-02 / UC-03 / UC-04：独立工作区 + 独立端口（28257）启动 `--serve`，注入样例块 `board_agents`（`work` online 带职责正文 + `reviewer` offline 自闭合）后实测：标题「公告板」，两行分别为 `work` + 绿色 `online` 徽章、`reviewer` + 灰色 `offline` 徽章，职责与 id、rev 均未显示。**通过**
- UC-05：紧随其后注入 `<board_agents rev="12"><unchanged/></board_agents>`，面板仍显示 `work online` / `reviewer offline` 两行，缓存内容为 `[{"text":"work","badge":"online"},{"text":"reviewer","badge":"offline"}]`。**通过**
- UC-07：`reviewer` 使用的是自闭合 `<agent …/>` 形式，解析正常。**通过**
- UC-09 / UC-10：注入 `<tools>` 三行（含 `bin/compare_xlsx` 无大小列）后实测只显示 3 条 `bin/…` 路径。**通过**
- UC-14：`node --check` 通过；`go build ./... && go vet ./...` 通过。**通过**
- UC-06 / UC-08 / UC-11 / UC-12 / UC-13：待人工在真实会话中验收。
## 根因修复（BUILD-1244）

用户反馈：面板里仍能看到 `board_agents rev="701"` 原文。排查结论与修复：

- **根因**：`agent/env_prune.go` 的 `tagNameOf` 返回 `line[1:end]`，把标签属性一起当成了块名。于是 `EnvBlocks()` 给出的 `Tag` 是 `board_agents rev="703"`，连带三个后果：
  1. 前端 `dynEnvLabel(tag)` 查不到映射 → 块标题直接显示原文；
  2. `tag === "board_agents"` 特判不命中 → 名册解析不生效；
  3. 块闭合检测 `"</"+name+">"` 永远匹配不上 → 该块 `raw` 会向下吞掉后续所有内容。
- **修复**：`tagNameOf` 只取第一个空白前的名字（`<board_agents rev="7">` → `board_agents`），顺带修正 `<sub_agents max="5">` 等带属性块的标题与边界。
- **配套**：`web/dyninfo.go` 新增 `applyBoardRoster`——面板展示时把 `<board_agents …><unchanged/></board_agents>` 精简态替换成当前名册（LLM 仍收到精简态），因此名册未变化时面板也能列出 agent。

### 新增用例与结果

- **UC-15（R）** 块名不带属性：`splitEnvBlocks` 对 `<board_agents rev="703">` 得到 `name == "board_agents"`，且 `raw` 只到 `</board_agents>`（不吞后续块）——`TestSplitEnvBlocksTagNames`。**通过**
- **UC-16（R）** 精简态展开：`applyBoardRoster` 在名册可用时把 `<unchanged/>` 块换成完整名册；名册为空时保留原文；已是明细的块不被重写——`TestApplyBoardRoster`。**通过**
- **UC-01 / UC-02 / UC-03 / UC-04（M）** 在正式实例（已接入 hub 的会话）重启后复核：标题为「公告板」，每行一个 agent 名称 + 彩色状态徽章。**待人工确认**

> 实测说明：用于本次验证的独立实例，其临时工作区没有 `bin/`、也未接入 hub 名册，环境块区不含 `<tools>` / `<board_agents>`，因此真实面板复核需在正式实例重启后进行。
