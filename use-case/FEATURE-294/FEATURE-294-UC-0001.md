# FEATURE-294-UC-0001: 系统提示词优化——参数说明补齐与规则冲突解决

## 测试目标

验证本次系统提示词优化的三个修复点是否全部生效：

1. `attempt_completion` 的 OpenAI 模式 Description 中缺少 `session_title`/`session_keywords` 参数说明
2. `reorganize_context` 的 OpenAI 模式 Description 缺少用法示例
3. `问题解决策略.md` 中"外科手术式改动"与"简单优先"的逻辑矛盾

---

## 验证方法

### 验证 1：attempt_completion OpenAI 模式参数说明

**检查方法**：`grep` 搜索

```bash
grep -A 3 "session_title" agent/tools.go | head -10
```

**预期结果**：
- `agent/tools.go` 中 `attempt_completion` 的 Description 文本（`Name: "attempt_completion"` 之后的 Description 字段）包含 `session_title` 和 `session_keywords` 的描述
- XML 模式（`i18n/zh_system.go` 和 `i18n/en_system.go`）的 Usage 维持不变

**验证命令**：
```bash
# 检查 OpenAI 模式 Description 中是否包含 session_title 和 session_keywords
grep -n "session_title\|session_keywords" agent/tools.go
# 预期：Description 中包含这两个参数的说明文本
```

---

### 验证 2：reorganize_context OpenAI 模式用法示例

经用户确认，OpenAI 模式 Description 已有足够详细的文字说明当前四个层次结构，无需额外补充 XML 示例。此项跳过。

---

### 验证 3：外科手术式改动与简单优先冲突解决
### 验证 3：外科手术式改动与简单优先冲突解决（已实施）

**检查方法**：对比文件内容

```bash
grep -A 2 "当当前任务范围内的代码过度复杂\|外科手术式改动\|简单优先" .clinerules/问题解决策略.md
```

**预期结果**：
- `.clinerules/问题解决策略.md` 中的"外科手术式改动"规则末尾增加冲突优先级说明
- 说明应明确指出当"简单优先"与"外科手术式改动"发生冲突时，应优先遵循"简单优先"

**验证命令**：
```bash
# 检查是否增加了冲突解决说明
grep -n "冲突\|优先\|优先遵循" .clinerules/问题解决策略.md
# 预期：至少有一行明确说明两者的优先级关系
```

---

## 通过标准

所有三个验证点均满足预期结果，视为测试通过。

---

## 验证 1：attempt_completion OpenAI 模式参数说明（已实施）

**检查方法**：`grep` 搜索

```bash
grep -n "session_title\|session_keywords" agent/tools.go | head -10
```

**预期结果**：
- `agent/tools.go` 中 `attempt_completion` 的 Description 文本（第 712-714 行附近）包含 `session_title` 和 `session_keywords` 的简短说明
- XML 模式（`i18n/zh_system.go` 和 `i18n/en_system.go`）的 Usage 维持不变

**验证命令**：
```bash
# 检查 OpenAI 模式 Description 中是否包含 session_title 和 session_keywords 说明
grep -n "session_title" agent/tools.go
# 预期：第 714 行附近包含 "session_title" 文本
```

---

## 通过标准

- 验证 1：Description 包含 `session_title` 和 `session_keywords` 说明 → 通过
- 验证 2：跳过 → 不验证
- 验证 3：rules 文件包含冲突优先级说明 → 通过

所有待验证的检查点均满足预期，视为测试通过。
### 验证 3：外科手术式改动与简单优先冲突解决

**检查方法**：对比文件内容

```bash
grep -A 2 "当当前任务范围内的代码过度复杂\|外科手术式改动\|简单优先" .clinerules/问题解决策略.md
```

**预期结果**：
- `.clinerules/问题解决策略.md` 中的"外科手术式改动"规则末尾增加冲突优先级说明
- 说明应明确指出当"简单优先"与"外科手术式改动"发生冲突时，应优先遵循"简单优先"

**验证命令**：
```bash
# 检查是否增加了冲突解决说明
grep -n "冲突\|优先\|优先遵循" .clinerules/问题解决策略.md
# 预期：至少有一行明确说明两者的优先级关系
```

---

## 通过标准

所有三个验证点均满足预期结果，视为测试通过。