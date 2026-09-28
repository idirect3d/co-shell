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
- 时间 / 实例 / BUILD：
- 结果：
