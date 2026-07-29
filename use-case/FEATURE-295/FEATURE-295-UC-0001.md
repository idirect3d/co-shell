# FEATURE-295-UC-0001: 默认系统提示词节列表验证

## 测试目的
验证三个内置模式（act/plan/research）的默认系统提示词节列表是否正确缩减，以及新增的 research 独立节配置函数。

## 测试步骤

### 1. 验证 DefaultBuiltInSections() 输出
**预期结果**：
```go
[]string{
    "Identity",
    "ToolUsage",
    "Capabilities",
    "Rules",
    "ExternalTools",
    "Environment",
}
```

### 2. 验证 DefaultActSections() 输出
**预期结果**：
```go
[]string{
    "Identity",
    "ToolUsage",
    "Capabilities",
    "Rules",
    "ExternalTools",
    "Environment",
}
```

### 3. 验证 DefaultPlanSections() 输出
**预期结果**：
```go
[]string{
    "Identity",
    "ToolUsage",
    "Capabilities",
    "Rules",
    "ExternalTools",
    "Environment",
}
```

### 4. 验证 DefaultResearchSections() 输出（新增）
**预期结果**：
```go
[]string{
    "Identity",
    "ToolUsage",
    "Capabilities",
    "Rules",
    "ExternalTools",
    "Environment",
}
```

### 5. 验证 DefaultWorkModes() 中 research 模式使用 DefaultResearchSections()
**预期结果**：research 模式的 Sections 字段引用 DefaultResearchSections()（而非 DefaultActSections()）

### 6. 验证编译通过
**操作**：`go build ./...`
**预期结果**：编译成功，无错误

### 7. 验证 vet 通过
**操作**：`go vet ./config/`
**预期结果**：无警告