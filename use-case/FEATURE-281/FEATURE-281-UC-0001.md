# FEATURE-281: write_to_file 工具描述添加大文件分次写入建议

## 测试用例编号
FEATURE-281-UC-0001

## 测试目的
验证 `write_to_file` 工具的 Description 中已正确添加大文件分次写入的 PERFORMANCE TIP。

## 前置条件
1. 代码已编译通过

## 测试步骤

### 步骤 1：编译验证
1. 执行命令:
   ```bash
   cd /Users/direct3d/github/co-shell && go build ./...
   ```
2. 期望结果：编译通过，无错误

### 步骤 2：启动 co-shell 并执行一次 LLM 调用（需开启 llm-log）
1. 开启 LLM 交互日志：
   ```
   :settings llm-log on
   ```
2. 输入一条简单指令让 LLM 响应（如 "hello"）
3. 检查日志文件，确认 LLM 收到的系统提示词中包含 write_to_file 的 PERFORMANCE TIP
4. 执行命令:
   ```bash
   grep "PERFORMANCE TIP" /Users/direct3d/github/co-shell/log/llm-interaction-*.log
   ```
5. 期望结果：日志文件中的系统提示词包含 `PERFORMANCE TIP` 及分次写入建议内容

## 预期结果
1. 代码编译通过
2. LLM 交互日志中 write_to_file 的描述末尾包含 PERFORMANCE TIP 内容
3. LLM 能够看到"大文件分多次 append 写入"的建议

## 实际结果
（留空，测试后填写）

## 备注
- 本次修改仅修改 `agent/tools.go` 第 375 行的 Description 字符串
- 不涉及其他代码文件的修改