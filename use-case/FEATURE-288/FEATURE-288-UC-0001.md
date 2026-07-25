# FEATURE-288 命令行参数与 REPL 参数补齐

## 测试目的
验证所有缺失的命令行参数、REPL `:set` 参数、CLI help 行、REPL `:help` 说明是否已正确补齐。

## 测试环境
- co-shell v0.6.0（任意平台）

---

## 测试 1：CLI 参数补齐

### 1.1 --mode 参数
```bash
co-shell --mode research --help 2>&1 | grep -- "--mode"
```
预期输出应包含 `--mode` 参数说明。

### 1.2 --thinking-enabled 参数
```bash
co-shell --thinking-enabled on --help 2>&1 | grep -- "--thinking-enabled"
```
预期输出应包含 `--thinking-enabled` 参数说明。

```bash
co-shell --thinking-enabled off --version
```
预期：不报错。

### 1.3 --reasoning-effort 参数
```bash
co-shell --reasoning-effort low --help 2>&1 | grep -- "--reasoning-effort"
```
预期输出应包含参数说明。

### 1.4 --max-retries 参数
```bash
co-shell --max-retries 5 --help 2>&1 | grep -- "--max-retries"
```
预期输出应包含参数说明。

### 1.5 --context-limit 参数
```bash
co-shell --context-limit 50 --help 2>&1 | grep -- "--context-limit"
```
预期输出应包含参数说明。

### 1.6 --shell-session-enabled 参数
```bash
co-shell --shell-session-enabled on --help 2>&1 | grep -- "--shell-session-enabled"
```
预期输出应包含参数说明。

### 1.7 --browser-enabled 参数
```bash
co-shell --browser-enabled on --help 2>&1 | grep -- "--browser-enabled"
```
预期输出应包含参数说明。

---

## 测试 2：`:set` 参数补齐

### 2.1 :set token-usage
启动 REPL 后：
```text
:set token-usage
```
预期：显示当前 Token 用量显示模式。

```text
:set token-usage on
```
预期：设置成功提示。

### 2.2 :set output-mode
```text
:set output-mode
```
预期：显示当前输出模式。

```text
:set output-mode compact
```
预期：设置成功提示。

---

## 测试 3：`--help` 输出补齐

```bash
co-shell --help 2>&1
```
预期输出中应包含：
- `--output-mode`
- `--token-usage`
- `--debug`
- `--body-add`
- `--init-capabilities`
- `--init-rules`

---

## 测试 4：REPL `:help` 补齐

启动 REPL 后：
```text
:help
```
预期输出中应包含：
- `:settings` 或 `:set`
- `:db`
- `:reset`
- `:list`
- `:vault`
- `:body-add`
- `:body-remove`
- `:body-display`
- `:new`
- `:continue`

---

## 测试 5：配置写入与持久化

```text
:set token-usage none
:set output-mode compact
```
验证 config.json 中已保存对应的配置值。

清除临时设置：
```text
:set token-usage on