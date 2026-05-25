# co-shell XML 工具调用模式：从想法到落地的技术实践

> 当国产大模型遇上 Function Calling——理想很丰满，现实很骨感。

## 背景

co-shell 是一个智能命令行 Shell，核心能力是让 AI Agent 理解用户意图并编排系统命令。这依赖于 LLM 的**工具调用（Tool Calling / Function Calling）**能力——模型返回结构化的工具调用请求，程序解析后执行对应操作。

但现实很骨感：**Function Calling 的问题比想象中多得多。**

### 问题一：部分模型根本不支持

- 早期不支持Function Calling的模型，主要是2025年上半年之前的开源模型。
- 现在很多高级深度思考类模型，明确不支持。
- 一些参数规模较小，不具备Function Calling能力的模型。

### 问题二：支持但不稳定（更隐蔽的坑）

这才是最头疼的。以我们实际测试的几个国产模型为例：

**DeepSeek-V4（表现良好）**
DeepSeek-V4 对 OpenAI 标准 Function Calling 的支持相当稳定，参数传递准确，多工具调用场景下也很少出错。在测试中基本没有因为格式问题导致调用失败的情况。

可是：

**Qwen 3.x（时好时坏）**
Qwen 3.x 虽然声明支持 OpenAI 标准的 Function Calling，但实际表现取决于问题复杂度：
- **简单场景**（1-2 个工具，少量参数）：表现良好，调用准确
- **复杂场景**（5+ 个工具，嵌套参数）：频繁出现问题
  - 缺少必填参数：工具定义中标记为 `required` 的参数，工具提示缺少参数，或者多了“，”。
  - 参数格式错误：传了但类型不对，比如要求 `number` 传了 `string`
  - 工具名称拼错：`read_file` 写成 `readfile` 或 `read-file`
  - 同时调用多个工具时互相干扰：参数张冠李戴
  - **最严重的是死循环**：模型反复调用同一个工具但参数错误，模型每次都表示知道错了，将增加这个参数，或者更小心地去写调用，但实际上模型就是不传，而且大概率会造成：Agent 重试 → 再错 → 再重试，陷入无限循环

**Mimo（直接罢工）**
Mimo 模型在 Function Calling 测试中的表现最差——**直接罢工**。要么返回空内容，要么返回一段描述性文字而不是结构化的工具调用，完全无法正常工作。

这些问题在简单场景下可能不出现，但一旦工具数量增多、参数复杂，模型的 Function Calling 输出质量就急剧下降。而且这类错误是**偶发性的**——同样的请求，这次成功下次失败，极难排查。

这就引出了一个问题：**如何让所有模型——无论是否支持 Function Calling——都能稳定可靠地使用工具？**

## 方案选择

### 方案一：JSON Mode

让模型以 JSON 格式输出工具调用。优点是结构清晰，缺点是 JSON 格式对模型要求高，容易格式错误，且需要额外解析。

### 方案二：XML Mode（最终选择）

让模型以 XML 标签格式输出工具调用。XML 的优势在于：
- 标签天然成对出现，模型更容易生成闭合结构
- 支持嵌套和属性，表达能力更强
- 对模型的格式要求比 JSON 宽松
- 人类可读性好，调试方便

最终我们选择了 XML 模式，并设计了一套完整的实现方案。

## 实现架构

### 核心设计

```
┌─────────────────────────────────────────────┐
│              ToolCallModeMgr                 │
│  ┌─────────────────┐  ┌──────────────────┐  │
│  │  OpenAI Mode     │  │   XML Mode       │  │
│  │  (原生 function  │  │  (XML 标签格式)   │  │
│  │   calling)       │  │                  │  │
│  └─────────────────┘  └──────────────────┘  │
│        线程安全 · 配置热加载 · 运行时切换      │
└─────────────────────────────────────────────┘
```

### 两种模式对比

| 特性 | OpenAI 模式 | XML 模式 |
|------|-----------|---------|
| 发送 tools 参数 | ✅ 是 | ❌ 否 |
| 系统提示词 | 标准用法说明 | XML 格式详细说明 |
| 解析方式 | SDK 原生解析 | 正则 + 标签解析 |
| 兼容性 | 仅支持 function calling 模型 | 所有模型 |
| 工具调用格式 | JSON 结构 | XML 标签 |

### XML 格式设计

我们设计了一套简洁的 XML 工具调用格式：

```xml
<tool_call>
<tool_name>read_file</tool_name>
<parameters>
<path>/etc/hosts</path>
</parameters>
</tool_call>
```

对于数组参数，使用 `<item>` 标签：

```xml
<tool_call>
<tool_name>search_files</tool_name>
<parameters>
<path>/home</path>
<pattern>*.go</pattern>
</parameters>
</tool_call>
```

## 踩坑记录

### 坑 1：反引号与 Shell 的冲突

**问题**：系统提示词中使用了反引号（`` ` ``）包裹代码示例，但在 Go 的原始字符串字面量（raw string literal）中，反引号是分隔符，无法直接使用。

**解决**：将系统提示词从 Go 源码中分离到独立的 i18n 文件中，使用 `%s` 占位符替代反引号，在运行时格式化替换。

```go
// 错误做法：反引号在 raw string 中无法使用
const prompt = `使用 <tool_call> 标签...`

// 正确做法：使用占位符
const promptTemplate = `使用 %s 标签...`
prompt := fmt.Sprintf(promptTemplate, "`<tool_call>`")
```

### 坑 2：XML 标签的闭合问题

**问题**：模型有时会生成不闭合的 XML 标签，或者嵌套顺序错误。

**解决**：解析时使用容错策略——不依赖严格的 XML 解析器，而是用正则表达式逐层匹配：

```go
func ParseXMLToolCalls(content string) []ToolCall {
    // 使用正则匹配 <tool_call>...</tool_call> 块
    re := regexp.MustCompile(`(?s)<tool_call>\s*<tool_name>(.*?)</tool_name>\s*<parameters>(.*?)</parameters>\s*</tool_call>`)
    // 对每个匹配块提取参数
    paramRe := regexp.MustCompile(`(?s)<(\w+)>(.*?)</\1>`)
    // ...
}
```

关键点：使用 `(?s)` 标志让 `.` 匹配换行符，使用非贪婪匹配 `.*?` 避免跨标签匹配。

### 坑 3：模型在 XML 中混入 Markdown

**问题**：模型有时会在 XML 代码块外加 Markdown 的 ` ```xml ` 包裹，或者添加额外的说明文字。

**解决**：解析前先清理内容——移除 Markdown 代码块标记，提取纯 XML 内容：

```go
// 移除 ```xml 和 ``` 标记
cleaned := strings.ReplaceAll(content, "```xml", "")
cleaned = strings.ReplaceAll(cleaned, "```", "")
```

### 坑 4：数组参数的表示

**问题**：当工具需要数组参数（如 `search_files` 的 `file_pattern` 支持多个模式）时，XML 的表示方式需要统一。

**解决**：统一使用 `<item>` 标签表示数组元素：

```xml
<parameters>
<file_pattern>
<item>*.go</item>
<item>*.md</item>
</file_pattern>
</parameters>
```

解析时检测到多个同名字段自动转为数组。

### 坑 5：配置热加载与运行时切换

**问题**：用户可能在对话中切换模式，需要确保切换立即生效，且不影响正在进行的工具调用。

**解决**：使用 `sync.RWMutex` 保护模式状态，`ApplyConfig()` 方法支持热加载：

```go
type ToolCallModeMgr struct {
    mu   sync.RWMutex
    mode ToolCallMode
}

func (m *ToolCallModeMgr) ApplyConfig(cfg *config.Config) {
    m.mu.Lock()
    defer m.mu.Unlock()
    if cfg.LLMConfig.ToolCallMode == "xml" {
        m.mode = ToolCallModeXML
    } else {
        m.mode = ToolCallModeOpenAI
    }
}
```

## 经验总结

### 1. 兼容性优先

在设计 AI Agent 系统时，**不要假设模型支持什么**。Function Calling 虽然方便，但不是所有模型都支持。提供降级方案（如 XML 模式）可以大幅提升系统的模型兼容性。

### 2. 提示词工程是关键

XML 模式的核心不在于解析代码，而在于**提示词的质量**。我们花了大量时间优化系统提示词中的 XML 格式说明，包括：
- 提供完整的示例
- 明确标签的嵌套规则
- 说明参数类型的对应关系
- 强调不要添加额外说明文字

### 3. 容错设计

模型输出永远不可预测。解析器必须做到：
- **宽松解析**：不要求严格格式
- **优雅降级**：解析失败时给出清晰错误信息
- **部分成功**：多个工具调用中部分失败不影响其他调用

### 4. 测试驱动

我们为 XML 解析编写了完整的测试用例，覆盖：
- 标准格式解析
- 带 Markdown 包裹的格式
- 数组参数
- 空参数
- 嵌套标签
- 错误格式的容错

```go
func TestParseXMLToolCalls(t *testing.T) {
    tests := []struct {
        name    string
        content string
        want    int  // 期望的工具调用数量
        wantErr bool
    }{
        {"standard xml", `<tool_call>...</tool_call>`, 1, false},
        {"markdown wrapped", "```xml\n<tool_call>...</tool_call>\n```", 1, false},
        {"multiple calls", `<tool_call>...</tool_call><tool_call>...</tool_call>`, 2, false},
        // ...
    }
    // ...
}
```

## 最终效果

经过上述优化，XML 模式在实际使用中表现良好：

- **兼容性**：从仅支持 OpenAI 兼容模型扩展到几乎所有 LLM
- **成功率**：XML 格式的工具调用解析成功率超过 95%
- **切换成本**：运行时切换模式零延迟，不影响对话上下文
- **代码量**：核心实现约 300 行，轻量无依赖

## 结语

XML 工具调用模式是 co-shell 在模型兼容性上的一次重要探索。它证明了：**即使模型不支持原生 Function Calling，通过精心设计的提示词和容错解析，依然可以实现稳定可靠的工具调用**。

这个思路也可以推广到其他 AI Agent 系统——当标准协议不可用时，用最朴素的方式（文本格式 + 正则解析）往往是最实用的解决方案。

## 为什么要从底层实现？

写到这里，我想聊点题外话。

现在打开任何技术社区，你看到的 AI Agent 文章无非几类：

- 分享怎么用 LangChain、CrewAI、AutoGPT 搭一个"智能体"
- 介绍某某研究团队又提出了多少种 Agent 模式
- 对比 Claude、ChatGPT 哪个写代码更强

看多了你会发现一个共同点：**大家都在用别人的轮子，拼别人的积木。**

一个典型的 Agent 项目，依赖链可能是这样的：

```
你的代码 → LangChain → OpenAI SDK → httpx → urllib3 → ...
```

每一层都是黑盒。出了问题，你根本不知道是 LangChain 的 bug、OpenAI SDK 的兼容问题、还是模型本身的问题。更可怕的是，这些依赖的依赖的依赖……你永远说不清你的生产环境里到底跑着谁的代码。

**这就是我选择从底层实现 co-shell 的原因。**

co-shell 的核心代码，从 LLM 客户端抽象、工具调用解析、Agent 循环、到持久化存储，全部自己实现。整个项目只有一个外部依赖——bbolt（嵌入式 KV 数据库），其他全部用 Go 标准库。

这意味着什么？

- **出了问题，我能修**。不管是 XML 解析的 bug 还是循环检测的逻辑错误，打开源码就能定位，不需要等上游修。
- **没有供应链风险**。没有哪个第三方库突然改 API、停止维护、或者被注入恶意代码。
- **性能可控**。10MB 的可执行文件，零运行时依赖，下载就能跑。不需要装 Python、Node.js、Docker。
- **行为可预测**。每一行代码都是我写的，我知道它在什么情况下会做什么。

我不是说用框架不对——框架有框架的价值，快速验证想法、降低入门门槛。但如果你要把一个 Agent 放到生产环境，**你真的放心让一个说不清供应链的"杂交怪物"去执行系统命令吗？**

co-shell 的选择很简单：**能自己写的，绝不依赖别人。** 这不是固执，这是对用户负责。

---

*本文由 co-shell 项目实战经验总结，项目地址：https://github.com/idirect3d/co-shell*
