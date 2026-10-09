// CoShellApp is the thin macOS shell that turns co-shell into a double-clickable
// application. It locates the embedded co-shell-hub binary, starts it on a free
// port with a per-user data directory, waits until its Web UI is reachable, then
// presents it in a WKWebView window. On quit it terminates the hub child process.
//
// Design notes:
//   - The shell only passes flags to the existing hub CLI, so no Go functional
//     code change is required (only the version string is bumped elsewhere).
//   - Ports are chosen by the shell (bind to port 0, read back the assignment)
//     and passed via --web-addr/--tcp-addr: hub auto-increments when a port is
//     taken, which would leave the shell unaware of the real port.
//   - hub stdout/stderr are redirected to ~/Library/Logs/co-shell-app/hub.log.

import Cocoa
import WebKit

// MARK: - Paths

private enum AppPaths {
    static let displayName = "co-shell"

    /// .../co-shell.app/Contents/Resources
    static var resources: URL { Bundle.main.resourceURL! }
    static var binDir: URL { resources.appendingPathComponent("bin") }

    static var coShellBinary: URL { binDir.appendingPathComponent("co-shell") }
    static var hubBinary: URL { binDir.appendingPathComponent("co-shell-hub") }

    /// ~/Library/Application Support/co-shell.app
    static var dataDir: URL {
        let base = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
        return base.appendingPathComponent("co-shell.app")
    }

    /// ~/Library/Logs/co-shell-app
    static var logDir: URL {
        let base = FileManager.default.urls(for: .libraryDirectory, in: .userDomainMask)[0]
        return base.appendingPathComponent("Logs/co-shell-app")
    }

    static var hubLog: URL { logDir.appendingPathComponent("hub.log") }
}

// MARK: - Port allocation

/// Asks the kernel for an unused TCP port on the loopback interface.
private func findFreePort() -> UInt16? {
    let fd = socket(AF_INET, SOCK_STREAM, 0)
    guard fd >= 0 else { return nil }
    defer { close(fd) }

    var addr = sockaddr_in()
    addr.sin_len = UInt8(MemoryLayout<sockaddr_in>.size)
    addr.sin_family = sa_family_t(AF_INET)
    addr.sin_port = 0 // let the kernel pick
    addr.sin_addr.s_addr = inet_addr("127.0.0.1")

    let bound = withUnsafePointer(to: &addr) { ptr in
        ptr.withMemoryRebound(to: sockaddr.self, capacity: 1) {
            bind(fd, $0, socklen_t(MemoryLayout<sockaddr_in>.size))
        }
    }
    guard bound == 0 else { return nil }

    var actual = sockaddr_in()
    var len = socklen_t(MemoryLayout<sockaddr_in>.size)
    let named = withUnsafeMutablePointer(to: &actual) { ptr in
        ptr.withMemoryRebound(to: sockaddr.self, capacity: 1) {
            getsockname(fd, $0, &len)
        }
    }
    guard named == 0 else { return nil }
    return UInt16(bigEndian: actual.sin_port)
}

// MARK: - App delegate

final class AppDelegate: NSObject, NSApplicationDelegate {
    private var window: NSWindow!
    private var webView: WKWebView!
    private var hub: Process?
    private var webPort: UInt16 = 0
    private var isQuitting = false

    func applicationDidFinishLaunching(_ notification: Notification) {
        buildMenu()
        prepareDirectories()
        buildWindow()

        guard let port = findFreePort() else {
            showError("无法分配本地端口。")
            return
        }
        webPort = port

        do {
            try startHub(webPort: port)
        } catch {
            showError("启动内嵌 hub 失败：\(error.localizedDescription)")
            return
        }

        waitUntilReady { [weak self] ready in
            guard let self = self else { return }
            if ready {
                self.loadUI()
            } else {
                self.showError("等待 hub Web UI 就绪超时（30 秒）。请查看日志：\(AppPaths.hubLog.path)")
            }
        }
    }

    func applicationWillTerminate(_ notification: Notification) {
        isQuitting = true
        guard let proc = hub, proc.isRunning else { return }
        proc.terminate() // SIGTERM first
        let deadline = Date().addingTimeInterval(3)
        while proc.isRunning && Date() < deadline {
            usleep(100_000)
        }
        if proc.isRunning {
            kill(proc.processIdentifier, SIGKILL)
        }
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { true }

    // MARK: - Setup

    private func buildMenu() {
        let main = NSMenu()
        let appItem = NSMenuItem()
        main.addItem(appItem)

        let appMenu = NSMenu()
        appMenu.addItem(withTitle: "关于 \(AppPaths.displayName)",
                        action: #selector(NSApplication.orderFrontStandardAboutPanel(_:)),
                        keyEquivalent: "")
        appMenu.addItem(.separator())
        appMenu.addItem(withTitle: "退出 \(AppPaths.displayName)",
                        action: #selector(NSApplication.terminate(_:)),
                        keyEquivalent: "q")
        appItem.submenu = appMenu

        NSApplication.shared.mainMenu = main
    }

    private func prepareDirectories() {
        for dir in [AppPaths.dataDir, AppPaths.logDir] {
            try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        }
    }

    private func buildWindow() {
        let rect = NSRect(x: 0, y: 0, width: 1280, height: 860)
        webView = WKWebView(frame: rect, configuration: WKWebViewConfiguration())

        window = NSWindow(contentRect: rect,
                          styleMask: [.titled, .closable, .miniaturizable, .resizable],
                          backing: .buffered,
                          defer: false)
        window.title = AppPaths.displayName
        window.contentView = webView
        window.center()
        window.makeKeyAndOrderFront(nil)
    }

    // MARK: - hub lifecycle

    private func startHub(webPort: UInt16) throws {
        let proc = Process()
        proc.executableURL = AppPaths.hubBinary
        proc.currentDirectoryURL = AppPaths.dataDir
        proc.arguments = [
            "--web-addr", "127.0.0.1:\(webPort)",
            "--tcp-addr", "127.0.0.1:\(webPort &+ 1)",
            "--config", AppPaths.dataDir.appendingPathComponent("hub-gateway.json").path,
            "--registry", AppPaths.dataDir.appendingPathComponent("hub-agents.json").path,
            "--co-shell-path", AppPaths.coShellBinary.path,
            "--no-api-key",
        ]

        let logHandle = try openHubLog()
        proc.standardOutput = logHandle
        proc.standardError = logHandle

        proc.terminationHandler = { [weak self] p in
            DispatchQueue.main.async {
                guard let self = self, !self.isQuitting else { return }
                self.showError("内嵌 hub 已意外退出（exit code \(p.terminationStatus)）。")
            }
        }

        try proc.run()
        hub = proc
    }

    private func openHubLog() throws -> FileHandle {
        let fm = FileManager.default
        if !fm.fileExists(atPath: AppPaths.hubLog.path) {
            fm.createFile(atPath: AppPaths.hubLog.path, contents: nil)
        }
        let handle = try FileHandle(forWritingTo: AppPaths.hubLog)
        handle.seekToEndOfFile()
        return handle
    }

    // MARK: - Web UI

    private func waitUntilReady(completion: @escaping (Bool) -> Void) {
        let deadline = Date().addingTimeInterval(30)
        guard let url = URL(string: "http://127.0.0.1:\(webPort)/") else {
            completion(false)
            return
        }

        func poll() {
            if Date() > deadline {
                completion(false)
                return
            }
            var request = URLRequest(url: url)
            request.timeoutInterval = 2
            request.cachePolicy = .reloadIgnoringLocalCacheData
            URLSession.shared.dataTask(with: request) { _, response, _ in
                let ok = (response as? HTTPURLResponse)?.statusCode == 200
                DispatchQueue.main.async {
                    if ok {
                        completion(true)
                    } else {
                        DispatchQueue.main.asyncAfter(deadline: .now() + 0.3, execute: poll)
                    }
                }
            }.resume()
        }
        poll()
    }

    private func loadUI() {
        guard let url = URL(string: "http://127.0.0.1:\(webPort)/") else { return }
        webView.load(URLRequest(url: url))
    }

    private func showError(_ message: String) {
        let html = """
        <!doctype html><html><head><meta charset="utf-8"><style>
        body{font:14px -apple-system,sans-serif;padding:40px;color:#222}
        h2{margin-top:0}code{background:#f4f4f4;padding:2px 5px;border-radius:3px}
        </style></head><body>
        <h2>co-shell 启动失败</h2>
        <p>\(message)</p>
        <p>hub 日志：<code>\(AppPaths.hubLog.path)</code></p>
        </body></html>
        """
        webView.loadHTMLString(html, baseURL: nil)
    }
}

// MARK: - Entry point

@main
struct CoShellAppMain {
    static func main() {
        let app = NSApplication.shared
        let delegate = AppDelegate()
        app.delegate = delegate
        app.setActivationPolicy(.regular)
        app.run()
    }
}
