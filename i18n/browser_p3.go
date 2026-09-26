// Author: L.Shuang
// Created: 2026-09-26
// Last Modified: 2026-09-26
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// FEATURE-556 Phase 3: messages of the additional browser tools (console,
// upload, PDF, network, tabs, mocking) in both languages.

package i18n

func init() {
	registerBrowserP3UsageZh()
	registerBrowserP3UsageEn()
	registerBrowserP3ResultsZh()
	registerBrowserP3ResultsEn()
}

// registerBrowserP3UsageZh registers the Chinese usage examples.
func registerBrowserP3UsageZh() {
	zhMessages[KeyToolUsageBrowserConsole] = `## browser_console
Description: 查看浏览器控制台自上次调用以来的新消息（JavaScript console 输出、未捕获异常、浏览器日志）。首次调用时按需开启采集，页面此前的输出由 Chrome 回放。
Parameters:
- meta (必需) 透明元数据对象，包含 intent/risk/risk_reason/affected_objects/progress。完整结构见系统提示词。
- clear (可选) 为 true 时丢弃已采集消息并返回空。

Usage:
<{XML_TAG_PREFIX}browser_console>
  <{XML_TAG_PREFIX}meta>
  <{XML_TAG_PREFIX}intent>汇总页面控制台错误</{XML_TAG_PREFIX}intent>
  <{XML_TAG_PREFIX}risk>low</{XML_TAG_PREFIX}risk>
</{XML_TAG_PREFIX}meta>
</{XML_TAG_PREFIX}browser_console>`

	zhMessages[KeyToolUsageBrowserUpload] = `## browser_upload
Description: 把工作区内的文件附加到指定文件输入框（DOM.setFileInputFiles），这是无需系统文件对话框填充文件输入框的唯一方式。相对路径按工作区解析，文件必须存在。
Parameters:
- meta (必需) 透明元数据对象，包含 intent/risk/risk_reason/affected_objects/progress。完整结构见系统提示词。
- selector (必需) 文件输入框的 CSS 选择器，如 input[type=file]。
- files (必需) 文件路径数组（工作区相对或绝对），顺序保留。

Usage:
<{XML_TAG_PREFIX}browser_upload>
  <{XML_TAG_PREFIX}meta>
  <{XML_TAG_PREFIX}intent>上传工作区文件到页面文件选择框</{XML_TAG_PREFIX}intent>
  <{XML_TAG_PREFIX}risk>low</{XML_TAG_PREFIX}risk>
</{XML_TAG_PREFIX}meta>
  <{XML_TAG_PREFIX}selector>input[type=file]</{XML_TAG_PREFIX}selector>
  <{XML_TAG_PREFIX}files>["input/sample.txt"]</{XML_TAG_PREFIX}files>
</{XML_TAG_PREFIX}browser_upload>`

	zhMessages[KeyToolUsageBrowserPdf] = `## browser_pdf
Description: 把当前页面导出为 PDF（Page.printToPDF）并保存到磁盘。
Parameters:
- meta (必需) 透明元数据对象，包含 intent/risk/risk_reason/affected_objects/progress。完整结构见系统提示词。
- path (可选) 输出路径（非绝对路径按工作区解析），默认 <workspace>/download/browser/<页面标题>.pdf。
- landscape / print_background (可选) 横向打印、打印 CSS 背景（后者默认 true）。
- paper_width / paper_height / scale (可选) 纸张尺寸（英寸）与缩放比例。

Usage:
<{XML_TAG_PREFIX}browser_pdf>
  <{XML_TAG_PREFIX}meta>
  <{XML_TAG_PREFIX}intent>把当前页面导出为 PDF</{XML_TAG_PREFIX}intent>
  <{XML_TAG_PREFIX}risk>low</{XML_TAG_PREFIX}risk>
</{XML_TAG_PREFIX}meta>
</{XML_TAG_PREFIX}browser_pdf>`

	zhMessages[KeyToolUsageBrowserNetwork] = `## browser_network
Description: 查看网络请求观测结果（状态码、MIME 类型、资源类型、失败原因）。首次调用时按需开启观测；CDP 不回放历史事件，需要观测某次加载时请先调用本工具再触发加载。
Parameters:
- meta (必需) 透明元数据对象，包含 intent/risk/risk_reason/affected_objects/progress。完整结构见系统提示词。
- only_failures (可选) 只返回失败请求。
- limit (可选) 最多列出条数（默认 20，最大 100）。

Usage:
<{XML_TAG_PREFIX}browser_network>
  <{XML_TAG_PREFIX}meta>
  <{XML_TAG_PREFIX}intent>报告当前页面的失败请求</{XML_TAG_PREFIX}intent>
  <{XML_TAG_PREFIX}risk>low</{XML_TAG_PREFIX}risk>
</{XML_TAG_PREFIX}meta>
  <{XML_TAG_PREFIX}only_failures>true</{XML_TAG_PREFIX}only_failures>
</{XML_TAG_PREFIX}browser_network>`

	zhMessages[KeyToolUsageBrowserTab] = `## browser_tab
Description: 管理浏览器标签：列出、新建、关闭或切换。切换会把 CDP 会话重新连接到目标页面，之后的工具都作用于该标签。
Parameters:
- meta (必需) 透明元数据对象，包含 intent/risk/risk_reason/affected_objects/progress。完整结构见系统提示词。
- action (必需) list / open / close / switch。
- url (可选) action=open 时的地址，缺省 about:blank。
- index (可选) list 结果中的 1 基序号（switch/close 使用）。
- target_id (可选) list 结果中的目标 id（switch/close 使用）。

Usage:
<{XML_TAG_PREFIX}browser_tab>
  <{XML_TAG_PREFIX}meta>
  <{XML_TAG_PREFIX}intent>新开标签并读取标题</{XML_TAG_PREFIX}intent>
  <{XML_TAG_PREFIX}risk>low</{XML_TAG_PREFIX}risk>
</{XML_TAG_PREFIX}meta>
  <{XML_TAG_PREFIX}action>open</{XML_TAG_PREFIX}action>
  <{XML_TAG_PREFIX}url>https://example.com</{XML_TAG_PREFIX}url>
</{XML_TAG_PREFIX}browser_tab>`

	zhMessages[KeyToolUsageBrowserMock] = `## browser_mock
Description: 用内存规则 mock 网络请求（Fetch 域）：命中的请求可返回固定响应（fulfill）、失败（fail）或放行（continue）。规则按 URL 子串匹配，或按 glob 匹配（glob=true 时 '*' 匹配任意字符，含 '/'）。仅当存在规则时才启用拦截。
Parameters:
- meta (必需) 透明元数据对象，包含 intent/risk/risk_reason/affected_objects/progress。完整结构见系统提示词。
- action (必需) add / list / clear / off。
- match / glob (可选) URL 子串或 glob 匹配串。
- rule_action (可选) fulfill / fail / continue，默认 fulfill（给出 error_reason 时默认 fail）。
- status / headers / body / base64_body (可选) fulfill 的响应内容。
- error_reason (可选) fail 的失败原因，如 Failed、Aborted、NameNotResolved。

Usage:
<{XML_TAG_PREFIX}browser_mock>
  <{XML_TAG_PREFIX}meta>
  <{XML_TAG_PREFIX}intent>mock 接口返回固定内容</{XML_TAG_PREFIX}intent>
  <{XML_TAG_PREFIX}risk>low</{XML_TAG_PREFIX}risk>
</{XML_TAG_PREFIX}meta>
  <{XML_TAG_PREFIX}action>add</{XML_TAG_PREFIX}action>
  <{XML_TAG_PREFIX}match>*/api/user</{XML_TAG_PREFIX}match>
  <{XML_TAG_PREFIX}glob>true</{XML_TAG_PREFIX}glob>
  <{XML_TAG_PREFIX}body>{"name":"mock"}</{XML_TAG_PREFIX}body>
</{XML_TAG_PREFIX}browser_mock>`
}

// registerBrowserP3UsageEn registers the English usage examples.
func registerBrowserP3UsageEn() {
	enMessages[KeyToolUsageBrowserConsole] = `## browser_console
Description: Report the browser console messages collected since the previous call (JavaScript console output, uncaught exceptions, browser log entries). Capture is enabled on the first call, and Chrome replays the output the page produced before that.
Parameters:
- meta (required) Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.
- clear (optional) When true, discard the collected messages and return nothing.

Usage:
<{XML_TAG_PREFIX}browser_console>
  <{XML_TAG_PREFIX}meta>
  <{XML_TAG_PREFIX}intent>Summarise the console errors of the page</{XML_TAG_PREFIX}intent>
  <{XML_TAG_PREFIX}risk>low</{XML_TAG_PREFIX}risk>
</{XML_TAG_PREFIX}meta>
</{XML_TAG_PREFIX}browser_console>`

	enMessages[KeyToolUsageBrowserUpload] = `## browser_upload
Description: Attach workspace files to a file input (DOM.setFileInputFiles), the only way to fill a file input without a native file dialog. Relative paths are resolved inside the workspace and must exist.
Parameters:
- meta (required) Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.
- selector (required) CSS selector of the file input, e.g. input[type=file].
- files (required) Array of workspace-relative or absolute file paths, order preserved.

Usage:
<{XML_TAG_PREFIX}browser_upload>
  <{XML_TAG_PREFIX}meta>
  <{XML_TAG_PREFIX}intent>Upload a workspace file to the page file input</{XML_TAG_PREFIX}intent>
  <{XML_TAG_PREFIX}risk>low</{XML_TAG_PREFIX}risk>
</{XML_TAG_PREFIX}meta>
  <{XML_TAG_PREFIX}selector>input[type=file]</{XML_TAG_PREFIX}selector>
  <{XML_TAG_PREFIX}files>["input/sample.txt"]</{XML_TAG_PREFIX}files>
</{XML_TAG_PREFIX}browser_upload>`

	enMessages[KeyToolUsageBrowserPdf] = `## browser_pdf
Description: Export the current page as PDF (Page.printToPDF) and save it to disk.
Parameters:
- meta (required) Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.
- path (optional) Output path (workspace-relative when not absolute); defaults to <workspace>/download/browser/<page title>.pdf.
- landscape / print_background (optional) Landscape orientation and CSS backgrounds (the latter defaults to true).
- paper_width / paper_height / scale (optional) Paper size in inches and page scale.

Usage:
<{XML_TAG_PREFIX}browser_pdf>
  <{XML_TAG_PREFIX}meta>
  <{XML_TAG_PREFIX}intent>Export the current page as PDF</{XML_TAG_PREFIX}intent>
  <{XML_TAG_PREFIX}risk>low</{XML_TAG_PREFIX}risk>
</{XML_TAG_PREFIX}meta>
</{XML_TAG_PREFIX}browser_pdf>`

	enMessages[KeyToolUsageBrowserNetwork] = `## browser_network
Description: Report the observed network requests (status, MIME type, resource type, failure reason). Observation is enabled on the first call; CDP does not replay past events, so call this tool before triggering the traffic you want to inspect.
Parameters:
- meta (required) Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.
- only_failures (optional) Only report failed requests.
- limit (optional) Maximum number of entries to list (default 20, maximum 100).

Usage:
<{XML_TAG_PREFIX}browser_network>
  <{XML_TAG_PREFIX}meta>
  <{XML_TAG_PREFIX}intent>Report the failed requests of the current page</{XML_TAG_PREFIX}intent>
  <{XML_TAG_PREFIX}risk>low</{XML_TAG_PREFIX}risk>
</{XML_TAG_PREFIX}meta>
  <{XML_TAG_PREFIX}only_failures>true</{XML_TAG_PREFIX}only_failures>
</{XML_TAG_PREFIX}browser_network>`

	enMessages[KeyToolUsageBrowserTab] = `## browser_tab
Description: Manage browser tabs: list, open, close or switch. Switching reconnects the CDP session to the selected page, so every later tool acts on that tab.
Parameters:
- meta (required) Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.
- action (required) list / open / close / switch.
- url (optional) Address for action=open (about:blank when omitted).
- index (optional) 1-based index from the list result (used by switch/close).
- target_id (optional) Target id from the list result (used by switch/close).

Usage:
<{XML_TAG_PREFIX}browser_tab>
  <{XML_TAG_PREFIX}meta>
  <{XML_TAG_PREFIX}intent>Open a new tab and read its title</{XML_TAG_PREFIX}intent>
  <{XML_TAG_PREFIX}risk>low</{XML_TAG_PREFIX}risk>
</{XML_TAG_PREFIX}meta>
  <{XML_TAG_PREFIX}action>open</{XML_TAG_PREFIX}action>
  <{XML_TAG_PREFIX}url>https://example.com</{XML_TAG_PREFIX}url>
</{XML_TAG_PREFIX}browser_tab>`

	enMessages[KeyToolUsageBrowserMock] = `## browser_mock
Description: Mock network requests with in-memory rules (Fetch domain): a matching request is answered with a canned response (fulfill), failed, or passed through. Rules match the URL as a substring, or as a glob when glob=true ('*' matches any characters including '/'). Interception is only enabled while a rule exists.
Parameters:
- meta (required) Transparency metadata object carrying intent/risk/risk_reason/affected_objects/progress. See the system prompt for the full structure.
- action (required) add / list / clear / off.
- match / glob (optional) URL substring or glob pattern.
- rule_action (optional) fulfill / fail / continue; default fulfil, or fail when error_reason is given.
- status / headers / body / base64_body (optional) Response content for fulfill.
- error_reason (optional) Failure reason for fail, e.g. Failed, Aborted, NameNotResolved.

Usage:
<{XML_TAG_PREFIX}browser_mock>
  <{XML_TAG_PREFIX}meta>
  <{XML_TAG_PREFIX}intent>Mock an API response</{XML_TAG_PREFIX}intent>
  <{XML_TAG_PREFIX}risk>low</{XML_TAG_PREFIX}risk>
</{XML_TAG_PREFIX}meta>
  <{XML_TAG_PREFIX}action>add</{XML_TAG_PREFIX}action>
  <{XML_TAG_PREFIX}match>*/api/user</{XML_TAG_PREFIX}match>
  <{XML_TAG_PREFIX}glob>true</{XML_TAG_PREFIX}glob>
  <{XML_TAG_PREFIX}body>{"name":"mock"}</{XML_TAG_PREFIX}body>
</{XML_TAG_PREFIX}browser_mock>`
}

// registerBrowserP3ResultsZh registers the Chinese tool results.
func registerBrowserP3ResultsZh() {
	zhMessages[KeyBrowserConsoleEmpty] = "浏览器控制台暂无新增消息。"
	zhMessages[KeyBrowserConsoleResult] = "浏览器控制台新增 %d 条消息（最多显示 %d 条）：\n%s"
	zhMessages[KeyBrowserMoreEntries] = "（另有 %d 条未列出）"
	zhMessages[KeyBrowserUploadResult] = "已上传 %d 个文件到 %s：\n%s"
	zhMessages[KeyBrowserPdfResult] = "PDF 已导出：%s（%d 字节）"
	zhMessages[KeyBrowserNetworkResult] = "已观测 %d 条请求（最多显示 %d 条）：\n%s"
	zhMessages[KeyBrowserNetworkEmpty] = "暂无网络请求记录。"
	zhMessages[KeyBrowserNetworkNoFailure] = "无失败请求。"
	zhMessages[KeyBrowserNetworkFailures] = "失败请求（最多列出 3 条）："
	zhMessages[KeyBrowserTabList] = "共 %d 个标签（* 为当前标签）：\n%s"
	zhMessages[KeyBrowserTabOpened] = "已打开新标签 %s：%s"
	zhMessages[KeyBrowserTabClosed] = "已关闭标签 %s。"
	zhMessages[KeyBrowserTabClosedCurrent] = "已关闭当前标签，请重新选择标签（下次操作会重新连接一个可用标签）。"
	zhMessages[KeyBrowserTabSwitched] = "已切换到标签 %s：%s（标题：%s）"
	zhMessages[KeyBrowserMockRules] = "当前 mock 规则 %d 条：\n%s"
	zhMessages[KeyBrowserMockEmpty] = "当前没有 mock 规则（请求不会被拦截）。"
	zhMessages[KeyBrowserMockCleared] = "已清除 mock 规则并关闭拦截，请求行为已恢复默认。"
	zhMessages[KeyBrowserToolError] = "操作失败：%s"
}

// registerBrowserP3ResultsEn registers the English tool results.
func registerBrowserP3ResultsEn() {
	enMessages[KeyBrowserConsoleEmpty] = "No new browser console messages."
	enMessages[KeyBrowserConsoleResult] = "Browser console has %d new message(s) (showing at most %d):\n%s"
	enMessages[KeyBrowserMoreEntries] = "(%d more not listed)"
	enMessages[KeyBrowserUploadResult] = "Uploaded %d file(s) to %s:\n%s"
	enMessages[KeyBrowserPdfResult] = "PDF exported: %s (%d bytes)"
	enMessages[KeyBrowserNetworkResult] = "Observed %d request(s) (showing at most %d):\n%s"
	enMessages[KeyBrowserNetworkEmpty] = "No network request recorded yet."
	enMessages[KeyBrowserNetworkNoFailure] = "No failed request."
	enMessages[KeyBrowserNetworkFailures] = "Failed requests (listing at most 3):"
	enMessages[KeyBrowserTabList] = "Found %d tab(s) ('*' marks the current tab):\n%s"
	enMessages[KeyBrowserTabOpened] = "Opened new tab %s: %s"
	enMessages[KeyBrowserTabClosed] = "Closed tab %s."
	enMessages[KeyBrowserTabClosedCurrent] = "Closed the current tab; select a tab again (the next call reconnects to an available tab)."
	enMessages[KeyBrowserTabSwitched] = "Switched to tab %s: %s (title: %s)"
	enMessages[KeyBrowserMockRules] = "Active mock rules: %d\n%s"
	enMessages[KeyBrowserMockEmpty] = "No mock rule is active (requests are not intercepted)."
	enMessages[KeyBrowserMockCleared] = "Cleared the mock rules and disabled interception; requests behave normally again."
	enMessages[KeyBrowserToolError] = "Operation failed: %s"
}
