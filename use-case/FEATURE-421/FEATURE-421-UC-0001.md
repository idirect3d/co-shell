# FEATURE-421 Web UI 信息块标题行文字增加阴影

## 背景

Web UI 信息块（LLM/THINK/TOOL/REPL 等）标题行 `.ev-head` 文字颜色为 `var(--fg-faint)`（较淡），部分字在某些场景下（如浅色背景、小字号）不够明显，影响可读性。需要给标题行文字增加约 2px 的阴影，提升对比度与可读性。

## 方案

修改 `web/static/style.css`：给 `.ev-head` 增加 `text-shadow`（约 2px 阴影），提升标题文字对比度，不影响布局。

## 验收标准

1. 信息块标题行文字（LLM/THINK/TOOL/REPL/COMMAND/SYSTEM 等）均显示约 2px 阴影。
2. 阴影不影响标题行布局（不改变尺寸、不遮挡相邻元素）。
3. 标题行文字在浅色/深色背景下均清晰可读。
4. 编译通过：`go build ./... && go vet ./... && go build -o work/co-shell .`

## 测试用例

### UC-001 信息块标题行文字有阴影
- 前置：Web UI 已渲染至少一个信息块（如 LLM 块）
- 步骤：检查 `.ev-head` 的 CSS 样式
- 期望：`.ev-head` 包含 `text-shadow` 属性，阴影约 2px

### UC-002 各类型信息块标题行均应用阴影
- 前置：Web UI 已渲染 LLM/THINK/TOOL/REPL 等多种类型信息块
- 步骤：逐一检查各类型信息块标题行
- 期望：所有类型信息块标题行均继承 `.ev-head` 的 `text-shadow` 样式

### UC-003 阴影不影响布局
- 前置：Web UI 已渲染信息块
- 步骤：对比增加阴影前后标题行尺寸与相邻元素间距
- 期望：标题行尺寸不变，相邻元素间距不变，无遮挡

### UC-004 标题行文字可读性提升
- 前置：Web UI 已渲染信息块
- 步骤：在浅色/深色背景下观察标题行文字
- 期望：标题行文字清晰可读，对比度较之前提升

### UC-005 编译通过
- 前置：代码修改完成
- 步骤：执行 `go build ./... && go vet ./... && go build -o work/co-shell .`
- 期望：编译、vet 全部通过，可执行码生成成功
