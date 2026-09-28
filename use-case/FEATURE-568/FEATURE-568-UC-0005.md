# FEATURE-568-UC-0005 supervisor 打回时不执行命令

- 分支：FEATURE-568（v0.82.0）｜类型：单元测试（边界）
- 目的：命令前移到「supervisor 审查通过之后」，因此当监督 LLM 打回（`approved=false`）导致任务返工重做时，**不应**执行命令，避免在成果未定稿时提前打开错误的文件。

## 步骤
1. 构造最小 Agent：`CompletionMode=active`、开启 supervisor（`EntryObject=true`、`MaxRetries` 足够大），顺序记录型 InteractionManager。
2. 第一次调用 `attemptCompletionTool`，args 含 `command`（`touch <tmp>/uc5.marker`）；使 supervisor 返回「不通过 + 反馈」。
3. 检查标记文件、`Ask` 调用情况与返回值。
4. 第二次调用（supervisor 改为通过），`Ask` 返回 `exit`，再检查标记文件与 `Ask` 时序。

## 期望
- 步骤 3：标记文件**不存在**（命令未执行）；`Ask` 未被调用；返回 supervisor 反馈文本（`approved=false` 分支），`completed=false`。
- 步骤 4：标记文件产生；`Ask` 被调用且进入时文件已存在；`completed=true`；返回消息追加命令输出段。

## 实测记录
- 时间 / 实例 / BUILD：2026-09-28 / 代码位置核对（无 LLM 桩）/ BUILD-1151
- 结果：⚠️ 未构造 supervisor 打回（`runSupervisorReview` 走“监督 LLM 不可用→强制放行”分支，无客户端时无法得到 `approved=false`）。已由**代码位置**保证：`if !approved { a.storeUserReply(feedback); return feedback, nil }`（`agent/tools.go:2448-2453`）位于命令执行块（`agent/tools.go:2460-2480`）**之前**，故打回时不执行命令；UC-0001 的顺序断言覆盖了「审查通过→弹框前执行」那一侧。
