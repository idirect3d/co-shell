# FEATURE-602 UC-0003：降级顺序、终止与本回合保持

> 分支：**FEATURE-602**　版本：**v0.94.0**　BUILD：**1211**
> 前置：本组用例均在「自动选择」开关**打开**时验证（开关默认关闭，见 UC-0001）。
> 判定口径：LLM 失败且成因可降级时，先耗尽当前模型的原地重试，再按模型列表优先级（降序）用「排在当前模型之后」的下一个可用模型重发同一请求；同一模型本回合内不重复尝试；直到最后一个可用模型仍失败才报错；成功降级后本回合保持使用该模型，不回退到已失败的上级模型。

## 背景

模型列表按 `Priority` 降序排列（`ModelManager.GetAllModels()`），当前生效模型是工作模式绑定模型或全局最高优先级启用模型（`Agent.selectModelForCall()`）。用户确认（2026-10-04 12:39）：先原地重试耗尽再降级；本回合保持降级模型。

## 变更点（预计）

| 位置 | 变更 |
|---|---|
| `agent/loop.go`（或 `agent.go`） | Agent 新增 `fallbackModelID string` 与 `fallbackTried map[string]bool`；`resetFallbackState()`；`nextFallbackModel()`（vision 场景仅取有 Vision 能力的模型）；`applyFallbackModel(id)` |
| `agent/agent.go` | `selectModelForCall()` 优先返回 `fallbackModelID` 对应的启用模型（vision 校验保留） |
| `agent/run_stream.go` | 回合开始时 `resetFallbackState()`；在非 `AllowsContextRemoval` 分支的原地重试耗尽后插入降级尝试（`errKind.Fallbackable() && AutoSelectEnabled()`），成功则 `continue` 重发；全部耗尽才报错并提示已尝试的模型 |

## 用例与结果

- **UC-01（S）按优先级降序降级** → 配置 3 个启用模型 A(P90)/B(P80)/C(P70)，当前为 A；A 失败（network）→ 选 B；B 也失败 → 选 C；C 失败 → 无可降级模型，报错。
- **UC-02（S）跳过已尝试模型** → 当前模型为 B（模式绑定，P80）时，降级候选为 C(P70)（A 优先级虽高但不可用/或按"排在后面"的语义不选更高优先级）；已失败过的模型不再被选中。
- **UC-03（S）仅启用模型参与** → 禁用（`enabled=false`）的模型不进入候选。
- **UC-04（S）vision 场景** → 带图片输入时，降级候选仅限具备 Vision 能力的启用模型。
- **UC-05（S）本回合保持** → 降级到 B 成功后，同一回合的后续迭代 `selectModelForCall()` 仍返回 B，即使 A 已恢复也不回退。
- **UC-06（S）新回合重置** → 下一次用户指令（新回合）`resetFallbackState()` 后重新从原选定模型开始。
- **UC-07（S）关闭开关不降级** → `AutoSelectEnabled()==false` 时，失败直接报错，不切换模型。
- **UC-08（S）不可降级错误直接报错** → `context-format`/`unknown` 不走降级分支。
- **UC-09（R）用户可见提示** → 每次降级发出 warn 事件（含"从 X 降级到 Y"与失败成因）；全部失败时报错信息包含已尝试模型列表。
- **UC-10（S）构建** → `go test ./agent/ -short -run TestFeature602` 通过。

## 不在本次范围

- 不改变模型优先级排序规则与置顶实现。
- 不做跨回合的降级记忆（每回合重置）。
- 不改动子 Agent / 监督者 / problem-solver 的模型选择。
