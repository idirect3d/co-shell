# FEATURE-293-UC-0001: no-tool-action 增强 — XML 工具调用反向识别

## 概述
验证增强的 XML 工具调用识别功能：
1. 阶段1：尾标签反向匹配 — 头标签拼错但尾标签是已知工具名时，产生 parse error
2. 阶段2：参数签名检测 — 头尾都不认识但内部参数标签 ≥ 1 个匹配已知参数名时，产生 parse error
3. hasToolAttempt 信号 — 有工具调用意图时走 parse-error-action 而非 no-tool-action(exit)

## 前置条件
1. 已编译 co-shell
2. 默认配置（no-tool-action=exit）

## 验证步骤

### 步骤 1：阶段1 — 尾标签反向匹配
输入到 XML 解析器：
```xml
<reed_file>
  <path>/tmp/test.txt</path>
  <intent>read file</intent>
</reed_file>
```
预期：
- 头标签 `reed_file` 不是已知工具
- 尾标签 `</reed_file>` 也不是已知工具名 → 进入阶段2

### 步骤 2：阶段2 — 参数签名检测
```xml
<reed_file>
  <path>/tmp/test.txt</path>
  <intent>read file</intent>
</file_read>
```
预期：
- 头标签 `reed_file` 不是已知工具
- 尾标签 `file_read` 也不是已知工具名（工具名是 read_file）
- 内部子标签：`path`, `intent`
- 已知参数名集合中包含 `path`, `intent`
- 匹配率 2/2 = 100% ≥ 30% → 触发 `_xml_parse_error`

### 步骤 3：正常 HTML 不误报
```html
<div>
  <p>hello world</p>
</div>
```
预期：
- 头 `div` 不是已知工具
- 尾 `</div>` 也不是已知工具名
- 内部子标签：`p`
- 已知参数名集合中不含 `p`
- 匹配率 0/1 = 0% < 30% → 静默跳过

### 步骤 4：头尾都不认识但参数匹配
```xml
<unkown_tag>
  <path>/tmp</path>
  <content>data</content>
</unkown_tag>
```
预期：
- 头 `unkown_tag` 不是已知工具
- 尾 `unkown_tag` 也不是已知工具名
- 内部子标签：`path`, `content`
- 已知参数名集合中包含 `path`, `content`
- 匹配率 2/2 = 100% ≥ 30% → 触发 `_xml_parse_error`

### 步骤 5：编译通过验证
```bash
go build ./...
# 预期：编译成功
```

## 预期结果
- 格式错误的 XML 工具调用被正确识别并产生 parse error
- 普通 HTML 内容不被误判
- parse error 走 parse-error-action（retry/prompt）而非 no-tool-action（exit）