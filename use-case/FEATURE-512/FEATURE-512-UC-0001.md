# FEATURE-512 测试用例 — ask_user 多问题收集能力升级

> 分支：FEATURE-512
> 版本：v0.52.0
> 关联任务：FEATURE-512
> 关联特性：FEATURE-388（统一交互模型 Interaction/InteractionManager）、FEATURE-438（选项补充信息）、FEATURE-452（固定键选项 - / +）、FEATURE-399（Web 虚拟键盘选项渲染）

## 背景

内置提问工具当前只支持「一个问题 + 单选 + 一条补充说明」：

- 工具注册与实现：`agent/tools.go`（`ask_followup_question`，参数 `question` 必需、`options` 可选）
- 统一交互模型：`agent/interaction.go`（`Interaction{Kind, Title, Options, Keys}` / `InteractionResult{Action, Value, Raw}`）
- Web 端：`web/session.go`（`WebIO.Ask` 下发 `kind:"interaction"` 消息）+ `web/static/app.js`（`showInteraction()` / `renderVirtualKeyboard()`）
- 终端：`agent/interaction.go`（`TerminalInteractionManager.askSelect`）

问题：LLM 一次需要确认多个决策点时，只能把多个问题硬塞进一个问题的选项里，选项不足以覆盖全部待确定项，用户也难以逐题作答。

## 设计方案

1. **工具更名**：`ask_followup_question` → `ask_user`；同步更新注册表、确认开关表、工具摘要/错误提示、i18n 用法示例与模式描述、相关单测。
2. **参数升级**：新增 `questions` 数组（1..N），每题：
   - `title`（必需）：题面
   - `options`（可选）：选项列表（2..N）
   - `multi`（可选，默认 false）：是否多选
   - `allow_note`（可选，默认 true）：是否允许在所选选项上补充说明
   - 兼容旧写法：`question` + `options` 归一化为单题（保证历史提示词/外部调用不失效）
3. **交互模型**：`agent/interaction.go` 新增 `Question`、`AnswerNote`、`QuestionAnswer` 结构与 `Interaction.Questions`、`InteractionResult.Answers` 字段，新增 `kind = "questions"`；`confirm` / `select` / `input` / `key` 语义不变。
4. **Web UI**：每题一个独立卡片（`Q1/Q2` 序号徽标 + 题面 markdown + 选项区 + 题间分隔线）；单选渲染 radio 样式、多选渲染 checkbox 样式；每个选项提供「补充说明」输入；底部「提交」按钮一次性回传全部答案，并显示已答进度（如 `提交 (2/3)`）。
5. **终端**：逐题渲染（`Q1/N` + 题面 + 选项），多选以逗号分隔作答（如 `1,3`），空格进入选项补充说明；全部答完后一次性回传。
6. **回传格式**：把每题答案格式化为可读文本（题面 + 所选项 + 各选项补充说明）写入 task instruction cache 并作为工具结果返回。
7. **留空语义**：某题未作答时允许提交，回传文本中该题标记为「（未作答）」，不阻塞用户。

---

## 用例清单

| 编号 | 场景 | 类型 |
|------|------|------|
| UC-0001 | XML 模式解析多题（questions/item 嵌套，含 options 子项） | 单元测试 |
| UC-0002 | JSON / OpenAI 模式解析多题参数 | 单元测试 |
| UC-0003 | 旧参数 `question` + `options` 兼容归一化为单题 | 单元测试 |
| UC-0004 | 每题 `multi` / `allow_note` 缺省值与显式值解析 | 单元测试 |
| UC-0005 | 构造出的 Interaction 结构正确（kind=questions、题数、选项、多选标记） | 单元测试 |
| UC-0006 | 答案回传文本格式（题面 + 选项 + 选项补充说明 + 未作答标记） | 单元测试 |
| UC-0007 | 用户取消 → 返回 CANCEL_AGENT | 单元测试 |
| UC-0008 | 工具名替换：ask_user 已注册且 ask_followup_question 不再注册 | 单元测试 |
| UC-0009 | 终端逐题作答（单选）→ 一次回传全部答案 | 单元测试 |
| UC-0010 | 终端多题作答（多选逗号分隔）→ 答案正确归位 | 单元测试 |
| UC-0011 | 终端选项补充说明（空格进入）→ 附加到对应选项 | 单元测试 |
| UC-0012 | Web 交互消息序列化：Questions / Answers 字段可正确 JSON 往返 | 单元测试 |
| UC-0013 | 无选项题目（自由输入题）在终端与答案回传中的处理 | 单元测试 |
| UC-0014 | 多题答案回传后 LLM 上下文可正确携带（task instruction cache） | 单元测试 |
| UC-0015 | Web 端渲染 2 题（单选 + 多选）：卡片分隔、控件形态正确 | 运行时 |
| UC-0016 | Web 端作答（单选 + 多选 + 选项补充说明）→ 提交 → 答案正确回传并被 LLM 采纳 | 运行时 |
| UC-0017 | Web 端部分题目留空 → 允许提交，回传含「（未作答）」标记 | 运行时 |
| UC-0018 | 终端完整流程：多题逐题作答 → 答案回传驱动任务继续 | 运行时 |
| UC-0019 | 旧格式（单题）调用回归：Web 端仍正常渲染与回传 | 运行时 |
| UC-0020 | 长问卷（5 题）Web 端布局与滚动正常，题间分隔清晰 | 运行时 |

---

## 单元测试用例

### UC-0001 XML 模式解析多题（questions/item 嵌套）

**输入**（XML 工具调用）

```xml
<ask_user>
  <meta>...</meta>
  <questions>
    <item>
      <title>用哪个数据库？</title>
      <options>
        <item>MySQL</item>
        <item>PostgreSQL</item>
      </options>
    </item>
    <item>
      <title>需要哪些能力？</title>
      <options>
        <item>读写分离</item>
        <item>自动备份</item>
      </options>
      <multi>true</multi>
    </item>
  </questions>
</ask_user>
```

**期望**
- `parseXMLChildrenToJSON` 解析后可被归一化器识别为 2 道题；
- Q1 题面「用哪个数据库？」，选项 `[MySQL, PostgreSQL]`，`multi=false`；
- Q2 题面「需要哪些能力？」，选项 `[读写分离, 自动备份]`，`multi=true`。

### UC-0002 JSON / OpenAI 模式解析多题参数

**输入**

```json
{
  "questions": [
    {"title": "用哪个数据库？", "options": ["MySQL", "PostgreSQL"]},
    {"title": "需要哪些能力？", "options": ["读写分离", "自动备份"], "multi": true, "allow_note": false}
  ]
}
```

**期望**：2 道题；Q2 的 `multi=true`、`allow_note=false` 被正确识别。

### UC-0003 旧参数兼容归一化为单题

**输入**：`{"question": "请选择处理方式", "options": ["立即执行", "稍后执行"]}`
**期望**：归一化为 1 道题；题面「请选择处理方式」，选项 2 个，`multi=false`、`allow_note=true`；不报参数缺失错误。

**变体**：仅 `{"question": "请补充说明"}`（无选项）→ 归一化为 1 道无选项题（自由输入）。

### UC-0004 multi / allow_note 缺省值与显式值

| 输入 | 期望 |
|------|------|
| 未提供 `multi` | 视为 false（单选） |
| 未提供 `allow_note` | 视为 true（允许补充说明） |
| `multi=true` | 多选 |
| `allow_note=false` | 该题不出现补充说明入口 |
| `multi` 为字符串 `"true"`（XML 解析产物） | 视为 true（需容错转换） |

### UC-0005 构造出的 Interaction 结构正确

**期望**：调用 `ask_user` 后传给 `InteractionManager.Ask` 的 `Interaction`：
- `Kind == InteractionQuestions`（值 `"questions"`）
- `len(Questions) == N`
- 每题的 `Title` / `Options` / `Multi` / `AllowNote` 与入参一致
- 每题保留固定键选项（`-` 再想想退出 / `+` 还有其他选项）

### UC-0006 答案回传文本格式

**输入**（用户作答）：Q1 选 MySQL；Q2 多选 `读写分离` + `自动备份`，且对「自动备份」补充「每天凌晨执行」；Q3 未作答。

**期望回传文本**（行为级要求，具体模板以 i18n 为准）
- 含每题题面，且题与题之间有明确分隔（换行/空行）；
- Q1 标注所选选项；
- Q2 标注两个所选项，并把补充说明关联到「自动备份」；
- Q3 标注为「（未作答）」；
- 文本写入 task instruction cache，并作为工具结果返回。

### UC-0007 用户取消

**输入**：交互返回 `ActionCancel`
**期望**：`ask_user` 返回错误 `CANCEL_AGENT`，与其他需要用户确认的工具行为一致。

### UC-0008 工具名替换

**期望**
- `buildTools` 中注册的工具名为 `ask_user`；
- 不再注册 `ask_followup_question`；
- `config` 默认确认开关表、`toolcall_mode.go` 工具用法示例、`tool_summary.go`、`tool_error.go` 中引用同步为 `ask_user`；
- `ask_user` 的 `meta` 参数仍为必需。

### UC-0009 终端逐题作答（单选）

**输入**（`mockUserIO.inputs`）：3 题分别为 `"1"` / `"2"` / `"1"`（每题单选，依次作答）
**期望**：3 题依次渲染（每题显示 `Qn/N` 与选项），输入被逐题消费，最终一次性回传包含 3 题答案的文本。

### UC-0010 终端多选作答（逗号分隔）

**输入**：多选题目输入 `"1,3"`（选项共 4 个）
**期望**：该题答案为选项 1 与选项 3 两项；非法编号（如 `"9"`）提示后重试，不吞掉输入。

### UC-0011 终端选项补充说明

**输入**：某题输入 `"2 需要兼容 PG15"`（选项序号 + 空格 + 补充文本）
**期望**：该题选择选项 2，且补充说明挂在该选项上；回传文本包含该补充说明。
**变体**：输入以空格开头（`" 模型选择待定"`）→ 进入自由补充模式，作为该题的自由输入。

### UC-0012 Web 交互消息 JSON 往返

**期望**：`Interaction{Kind:"questions", Questions:[...]}` 序列化后，`Questions` 字段完整（含 `multi` / `allow_note` / `options`）；`InteractionResult{Answers:[...]}` 从浏览器回传后反序列化，`Answers` 的 `selected` / `notes` 与提交内容一致。

### UC-0013 无选项题目（自由输入题）

**期望**
- 终端：该题直接等待文本输入，输入内容作为该题答案；
- Web：该题渲染为文本输入框；
- 回传文本中该题以「回答: xxx」形式呈现。

### UC-0014 多题答案进入 LLM 上下文

**期望**：`ask_user` 返回后，task instruction cache 含全部答案文本；后续 LLM 上下文消息中可检索到该文本（沿用 `storeUserReply` 路径）。

---

## 运行时用例

> 通用前置：使用 `co-shell --serve --port <port>` 启动（不带 `--serve` 会自动打开系统浏览器并占用单客户端连接），浏览器连接后确认状态为「已连接」。
> 触发方式优先级：① 让 Agent 在真实任务中调用 `ask_user`；② 若 LLM 触发不稳定，用 `web/session_test.go` 风格的集成测试（伪造 `interaction_answer`）做等价验证。

### UC-0015 Web 端渲染 2 题（单选 + 多选）

**步骤**
1. 触发一次 `ask_user`，下发 2 题：Q1 单选（2 个选项）、Q2 多选（3 个选项）。
2. 观察交互区渲染。

**期望**
- 出现 2 个独立标题（`Q1` / `Q2` 序号清晰），两题之间有可见分隔（分隔线或明显留白），不会混成一段；
- Q1 的选项呈现为单选形态（radio：同一时刻只能选中一个）；
- Q2 的选项呈现为多选形态（checkbox：可同时选中多个）；
- 每题下方有「补充说明」入口，底部有「提交」按钮。

### UC-0016 Web 端作答并提交 → 答案正确回传

**步骤**
1. 在 UC-0015 的问卷中：Q1 选中「PostgreSQL」；Q2 勾选「读写分离」与「自动备份」，并对「自动备份」点击补充说明，输入「每天凌晨执行」。
2. 点击「提交」。
3. 观察事件流中工具结果与 Agent 后续回复。

**期望**
- 提交后交互区关闭，事件流显示用户答案（回显）；
- `ask_user` 工具结果包含：Q1 = PostgreSQL；Q2 = 读写分离、自动备份，且「每天凌晨执行」关联到「自动备份」；
- 未填写的题若存在，标注「（未作答）」；
- Agent 能据答案继续推进任务（不重复提问同一问题）。

### UC-0017 Web 端部分题目留空

**步骤**：下发 3 题，仅作答 2 题，直接点击「提交」。

**期望**
- 允许提交，不被阻塞；
- 未作答的题在回传文本中标注「（未作答）」；
- 提交按钮上的计数（如 `提交 (2/3)`）与实际作答数一致。

### UC-0018 终端完整流程（逐题作答）

**步骤**
1. 在 TUI（`co-shell run` 或 REPL）触发一次 3 题问卷：Q1 单选、Q2 多选、Q3 自由输入。
2. 依次输入：`1`、`1,3`、`请保留旧表`。
3. 观察终端输出与 Agent 后续动作。

**期望**
- 每题独立渲染，题面带 `Qn/N` 标注，问题之间清晰分隔；
- 多选以逗号分隔被正确解析为两项；
- 作答完成后一次性把 3 题答案交给 LLM，Agent 据答案继续推进。

### UC-0019 旧格式单题调用回归（Web）

**步骤**：触发一次旧参数形式的调用（`question` + `options`，2 个选项）。

**期望**
- 仍以单题问卷渲染（等价于 1 题问卷），选项可点选并提交；
- 答案回传内容与改造前语义一致（含所选选项文本）；
- 固定键 `-`（再想想退出）与 `+`（还有其他选项）仍可用。

### UC-0020 长问卷（5 题）布局与滚动

**步骤**：触发一次 5 题问卷（含单选、多选、自由输入各若干），观察交互区高度与滚动。

**期望**
- 交互区可滚动查看全部 5 题，不遮挡底部输入框；
- 题间分隔一致，序号连续（Q1..Q5）；
- 提交后交互区正常收起，事件流回到最新位置。

---

## 验收汇总（进入编码阶段的门槛）

- 单元用例 UC-0001 ~ UC-0014 全部以 Go 单测落地并通过（`go test ./agent/ ./web/`）；
- 运行时用例 UC-0015 ~ UC-0020 在本地实例上逐条实测通过（保留截图/日志证据）；
- 旧调用格式（`question` + `options`）不回归；
- `go build ./... && go vet ./...` 全绿；
- 多语言：新增文案（提交按钮、补充说明、未作答标记、题号模板等）在 zh / en 两个语言包中均存在。
