# co-shell 隐私政策 / Privacy Policy

最后更新：2026-10-10 · Last updated: 2026-10-10

适用产品：co-shell（含 Mac App Store 上架的沙箱版，bundle id `com.coshell.app.mas`，以及同源代码构建的命令行版）。
This policy covers co-shell, including the sandboxed Mac App Store build (`com.coshell.app.mas`) and the command-line build from the same source.

> 一句话概括：**co-shell 是一个完全运行在您自己设备上的本地 AI Agent 应用。开发者不运营任何后端服务器，不收集、不上传、不追踪您的任何数据。** 只有当您自己配置了模型服务时，您的提示词与上下文才会由您的设备直接发往您所选的服务商。
>
> In one sentence: **co-shell is a local AI agent application that runs entirely on your own Mac. The developer operates no backend servers and does not collect, upload or track any of your data.** Your prompts reach a model provider only when you configure one yourself — sent directly from your Mac to the endpoint you chose.

---

## 1. 开发者不收集任何数据 / We collect nothing

- 无账号体系：您无需注册、无需登录即可使用全部功能。
- 无服务端：开发者不运营服务器，因此不存在开发者侧的用户数据副本。
- 无统计、无埋点、无追踪、无广告 SDK，不使用第三方分析服务。
- 应用内声明的隐私清单（`PrivacyInfo.xcprivacy`）与该口径一致：

| 声明项 | 值 |
|---|---|
| `NSPrivacyTracking` | `false` |
| `NSPrivacyTrackingDomains` | 空 |
| `NSPrivacyCollectedDataTypes` | 空（不采集任何数据类型） |

App 隐私问卷的填写口径为「不收集数据」。

---

## 2. 数据保存在您自己的设备上 / Everything is stored on your device

co-shell 的运行数据全部存放在本机，不上传：

- **配置**（含模型地址、模型名、您填写的 API Key）：命令行版默认 `~/.co-shell/config.json`；Mac App Store 沙箱版存放在该 App 的容器内。
- **会话记录、记忆、任务计划、定时任务等**：本地键值数据库（默认 `db/co-shell.db`）。
- **日志**：本地 `log/` 目录。
- **您指定工作区内的文件**：由您自己的项目目录承载，co-shell 不另存副本。

API Key 等敏感值在界面展示时会脱敏；它们只用于您本机向您配置的端点发起请求。

---

## 3. 会离开设备的数据（由您决定） / Data that leaves your device (your choice)

co-shell 本身内置引擎，但**模型推理由您自行配置的服务商完成**。因此：

- 您输入的提示词、会话上下文，以及 agent 按您指示读取到的文件内容，会在执行任务时由您的设备**直接发送**给您在配置中指定的模型端点。
- 请求不经过开发者的任何服务器；开发者无法看到这些内容。
- 该端点的数据处理方式适用**该服务商自己的隐私政策**，请按其条款评估。
- 如果您不使用云端模型（例如配置本地推理服务），则上述内容不会离开您的设备或局域网。

除此之外，只有当您**主动启用**某些可选功能时，才会产生对应的联网行为，且只连接您自己指定的地址，例如：自建/指定的 hub 网关、自行添加的 MCP 服务器、浏览器自动化（连接本机已安装的浏览器）、移动端或飞书等桥接。这些功能默认不启用，且始终由您配置与关闭。

---

## 4. 文件与命令访问 / File and command access

co-shell 的核心功能就是**在您的指示下**帮您处理您自己的文件：

- 它不会自行扫描、索引或上传您的磁盘；它只在您本次会话明确要求的任务范围内读写文件。
- 系统命令只在本地、以您自身的账号权限、且仅在您本次会话的指示下执行。
- Mac App Store 沙箱版在启用 App Sandbox 的同时，申请了一项文件访问临时例外
  （`com.apple.security.temporary-exception.files.home-relative-path.read-write`，值 `/`）：
  因为任务目标是您**运行时**选定的项目目录，构建期无法预先声明更窄的路径；若只允许访问 App 自己的容器，该产品将无法处理您正在询问的文件。
- 访问以下隐私敏感 API 的理由（已在隐私清单中声明）：文件时间戳（展示文件时间）、系统启动时间（计算时间间隔）、磁盘空间（空间检查）、`UserDefaults`（保存应用偏好设置）。这些调用均只在本机进行。

---

## 5. 数据保留与删除 / Retention and deletion

- 数据保留多久，完全由您决定：它们只是您设备上的文件。
- 删除方式：删除对应的本地数据库文件与配置文件（沙箱版为 App 容器内数据）；卸载 App 并清除其容器数据即可完全移除。
- 由于开发者没有服务端副本，无需向开发者申请删除。

---

## 6. 儿童 / Children

本应用不面向儿童，也不收集任何年龄信息；由于不采集任何数据，本应用不会收集儿童的个人信息。

---

## 7. 政策变更 / Changes to this policy

本政策的更新随应用版本发布，最新版本始终位于本页面。若发生实质性变更，会在应用更新说明中提及。

---

## 8. 联系方式 / Contact

- 仓库与问题反馈：<https://github.com/idirect3d/co-shell/issues>
- 源码：<https://github.com/idirect3d/co-shell>

---

# English

Last updated: 2026-10-10

## 1. We collect nothing

- No account: no sign-up, no sign-in, all features are available offline-of-any-service.
- No backend: the developer operates no servers, so no copy of your data exists on the developer side.
- No analytics, no telemetry, no tracking, no advertising SDKs, no third-party analytics.
- The bundled privacy manifest (`PrivacyInfo.xcprivacy`) states the same: `NSPrivacyTracking = false`, `NSPrivacyTrackingDomains = []`, `NSPrivacyCollectedDataTypes = []`. The App Privacy questionnaire is answered as "Data Not Collected".

## 2. Everything is stored on your device

- Configuration (model endpoint, model name, your API key): `~/.co-shell/config.json` for the command-line build; inside the app container for the sandboxed Mac App Store build.
- Sessions, memory, task plans, scheduled jobs: a local key-value database (default `db/co-shell.db`).
- Logs: the local `log/` directory.
- Files in your workspace: they stay in your own project directories; co-shell keeps no separate copy.

Sensitive values such as API keys are masked in the UI and are used only for requests from your Mac to the endpoint you configured.

## 3. Data that leaves your device (your choice)

Model inference is performed by a provider **you** configure:

- Your prompts, the session context, and the file content the agent reads on your instruction are sent **directly from your Mac** to the model endpoint you configured.
- Those requests do not pass through any developer-operated server; the developer cannot see them.
- That endpoint's handling of the data is governed by **that provider's own privacy policy**.
- If you configure a local inference service instead, the content never leaves your machine or your local network.

Beyond that, network activity only happens when you explicitly enable an optional feature, and only toward an address you specify — for example a self-hosted hub gateway, an MCP server you added, browser automation (connecting to a browser installed on your Mac), or a mobile/Feishu bridge. These are off unless you set them up, and you can turn them off at any time.

## 4. File and command access

Working on your own files, at your direction, is the product's core function:

- co-shell never scans, indexes or uploads your disk on its own; it touches files only within the task you explicitly requested in that session.
- System commands run locally, under your own account, and only as directed by you in that session.
- The sandboxed Mac App Store build runs with App Sandbox enabled and requests one file-access temporary exception (`com.apple.security.temporary-exception.files.home-relative-path.read-write`, value `/`): the target directory of any task is chosen by you at runtime, so no narrower pre-declared path can be given at build time. Restricted to its own container, the app could not touch the very files you are asking about.

## 5. Retention and deletion

Retention is entirely up to you — the data is simply files on your device. Delete the local database and configuration files (inside the app container for the sandboxed build), or uninstall the app and clear its container, and the data is gone. There is no server-side copy, so no deletion request to the developer is needed.

## 6. Children

The app is not directed to children and collects no age information; because it collects no data at all, it collects no personal information from children.

## 7. Changes to this policy

Updates to this policy ship with app releases; the current version always lives on this page. Material changes are noted in the release notes.

## 8. Contact

- Issues: <https://github.com/idirect3d/co-shell/issues>
- Source: <https://github.com/idirect3d/co-shell>
