# FEATURE-293-UC-0001: no-tool-action 默认值改为 exit

## 概述
验证 no-tool-action 的默认值已从 "retry" 改为 "exit"。

## 前置条件
1. 已编译 co-shell
2. 无自定义 config.json（或 config.json 中不含 no-tool-action 配置）

## 验证步骤

### 步骤 1：验证默认配置
```bash
go run . --help 2>&1 | head -1
# 预期：启动正常
```

### 步骤 2：验证源代码默认值
```bash
grep -n 'NoToolAction' config/config.go
# 预期：
#   NoToolAction string `json:"no_tool_action"`
#   NoToolAction:               "exit",
```

### 步骤 3：编译通过验证
```bash
go build ./...
# 预期：编译成功，无错误
```

### 步骤 4：无 config.json 时默认行为验证
```bash
# 确认当前目录无 config.json
ls config.json 2>/dev/null && echo "exists" || echo "not exists"
# 预期：not exists

# 验证新生成的默认配置为 exit
go run . -c 'echo test' --no-tool-action 2>&1 || true
# 或用 :set 命令验证
go run . -c ':set' 2>&1 | grep no-tool-action
# 预期：no-tool-action = exit
```

## 预期结果
- 默认无 config.json 时，no-tool-action = exit
- 编译通过
- LLM 在 0 个工具调用时默认退出迭代循环