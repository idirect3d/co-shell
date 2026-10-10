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
//   - The title-bar strip is painted by the shell, not by the page: the window
//     uses .fullSizeContentView with a transparent title bar, its content view
//     is split into a 28 pt strip on top and the web view right below, and the
//     strip is repainted with the page colour reported by the injected probe.

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
    ///
    /// Under the App Sandbox (the Mac App Store build) the standard directories
    /// resolve inside the app container, so this becomes
    /// `~/Library/Containers/<bundle id>/Data/Library/Application Support/co-shell.app`.
    /// That is intended: the layout is unchanged, only the root moves.
    static var dataDir: URL {
        let base = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
        return base.appendingPathComponent("co-shell.app")
    }

    /// ~/Library/Logs/co-shell-app
    ///
    /// Container-redirected under the App Sandbox exactly like `dataDir`.
    /// No other process — including a terminal — can read inside the container,
    /// so the app exposes a menu entry to open this directory; otherwise the hub
    /// log would be unreachable while troubleshooting.
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

// MARK: - Title-bar strip

/// Title-bar metrics and the colour used until the page reports its own.
private enum Titlebar {
    /// Height of the title bar of a window without a toolbar (standard macOS).
    static let height: CGFloat = 28

    /// Name of the web -> shell message carrying the page background colour.
    static let messageName = "coShellThemeColor"

    /// Dark page background, shown until the probe reports the real colour.
    static let fallbackColor = NSColor(srgbRed: 11 / 255, green: 14 / 255, blue: 20 / 255, alpha: 1)

    /// Font size of the centred wordmark painted by the shell (FEATURE-627).
    static let wordmarkFontSize: CGFloat = 12.5

    /// Secondary text colour for the strip: dark text on a light strip, light
    /// text on a dark one, so the wordmark stays readable in both themes.
    static func secondaryTextColor(on background: NSColor) -> NSColor {
        let base = background.usingColorSpace(.sRGB) ?? fallbackColor
        let luma = 0.2126 * base.redComponent + 0.7152 * base.greenComponent + 0.0722 * base.blueComponent
        return luma > 0.5
            ? NSColor.black.withAlphaComponent(0.45)
            : NSColor.white.withAlphaComponent(0.45)
    }

    /// Parses the CSS colour notations the probe can report (`#rgb`, `#rrggbb`,
    /// `rgb()` and `rgba()`).
    static func color(fromCSS css: String) -> NSColor? {
        let text = css.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()

        if text.hasPrefix("#") {
            var hex = String(text.dropFirst())
            if hex.count == 3 {
                hex = hex.map { "\($0)\($0)" }.joined()
            }
            guard hex.count == 6 || hex.count == 8, let value = UInt64(hex, radix: 16) else { return nil }
            let hasAlpha = hex.count == 8
            func component(_ shift: Int) -> CGFloat {
                CGFloat((value >> UInt64(shift)) & 0xFF) / 255
            }
            return NSColor(srgbRed: component(hasAlpha ? 24 : 16),
                           green: component(hasAlpha ? 16 : 8),
                           blue: component(hasAlpha ? 8 : 0),
                           alpha: hasAlpha ? component(0) : 1)
        }

        guard text.hasPrefix("rgb"),
              let open = text.firstIndex(of: "("),
              let close = text.lastIndex(of: ")") else { return nil }
        let parts = text[text.index(after: open)..<close]
            .split(whereSeparator: { $0 == "," || $0 == " " || $0 == "/" })
            .map(String.init)
        guard parts.count >= 3 else { return nil }

        func channel(_ raw: String) -> CGFloat? {
            if raw.hasSuffix("%") {
                guard let percent = Double(raw.dropLast()) else { return nil }
                return CGFloat(percent) / 100
            }
            // Computed styles always serialise rgb() with 0...255 channels;
            // NSColor clamps its components to 0...1 and turns anything larger
            // into a near-black colour, so normalise here.
            guard let value = Double(raw) else { return nil }
            return CGFloat(value) / 255
        }
        guard let red = channel(parts[0]), let green = channel(parts[1]), let blue = channel(parts[2]) else {
            return nil
        }
        let alpha = parts.count >= 4 ? (channel(parts[3]) ?? 1) : 1
        return NSColor(srgbRed: red, green: green, blue: blue, alpha: alpha)
    }
}

/// The strip that fills the title-bar area. It stays a window drag handle
/// (mouseDownCanMoveWindow) so the transparent title bar keeps working, and it
/// paints the product name centred on it (FEATURE-627).
final class TitlebarStripView: NSView {
    override var mouseDownCanMoveWindow: Bool { true }

    /// Centred wordmark; a label (non-editable, non-selectable) so it never
    /// takes focus away from the page.
    private let wordmark = NSTextField(labelWithString: "co-shell")

    override init(frame frameRect: NSRect) {
        super.init(frame: frameRect)
        wordmark.font = .systemFont(ofSize: Titlebar.wordmarkFontSize, weight: .medium)
        wordmark.alignment = .center
        wordmark.translatesAutoresizingMaskIntoConstraints = false
        addSubview(wordmark)
        NSLayoutConstraint.activate([
            wordmark.centerXAnchor.constraint(equalTo: centerXAnchor),
            wordmark.centerYAnchor.constraint(equalTo: centerYAnchor),
        ])
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) has not been implemented") }

    /// The label must not swallow clicks, otherwise the middle of the strip
    /// would stop dragging the window.
    override func hitTest(_ point: NSPoint) -> NSView? {
        let hit = super.hitTest(point)
        return hit === wordmark ? self : hit
    }

    /// Paints the strip with the page colour and adapts the wordmark to it.
    func setBarColor(_ color: NSColor) {
        wantsLayer = true
        layer?.backgroundColor = color.cgColor
        wordmark.textColor = Titlebar.secondaryTextColor(on: color)
    }
}

/// Measures the background colour of the page area right below the strip and
/// posts it to the shell, re-posting whenever the page theme may have changed.
private let themeProbeScript = """
(function () {
  var NAME = 'coShellThemeColor';
  var last = null;

  function alphaOf(c) {
    if (c.indexOf('rgba(') !== 0) return 1;
    var parts = c.slice(5, c.length - 1).split(',');
    if (parts.length < 4) return 1;
    var a = parseFloat(parts[3]);
    return isNaN(a) ? 1 : a;
  }

  // Solid background colour of one element, or null when it is see-through.
  function solidBackground(el, win) {
    var cs = win.getComputedStyle(el);
    var bg = cs.backgroundColor;
    if (!bg || bg === 'transparent' || bg.indexOf('color(') === 0) return null;
    if (alphaOf(bg) < 0.9) return null;
    var img = cs.backgroundImage;
    if (img && img !== 'none') return null;
    return bg;
  }

  // Walks up from the element painted at the very top of a document until a
  // solid background is found: that is the colour shown below the strip. The
  // hub page hosts the agent UI in a same-origin iframe, whose element itself
  // reports the browser's default white, so descend into the frame first.
  function colourAt(doc, x, y, depth) {
    if (!doc || !doc.defaultView || depth > 4) return null;
    var el = doc.elementFromPoint(Math.round(x), Math.round(y));
    while (el) {
      if (el.tagName === 'IFRAME') {
        var inner = null;
        try { inner = el.contentDocument; } catch (e) { inner = null; }
        if (inner) {
          var box = el.getBoundingClientRect();
          var nested = colourAt(inner, x - box.left, y - box.top, depth + 1);
          if (nested) return nested;
        }
      }
      var own = solidBackground(el, doc.defaultView);
      if (own) return own;
      el = el.parentElement;
    }
    return null;
  }

  function measure() {
    return colourAt(document, window.innerWidth / 2, 4, 0);
  }

  function report() {
    try {
      var c = measure();
      if (c && c !== last) {
        last = c;
        window.webkit.messageHandlers[NAME].postMessage(c);
      }
    } catch (e) {}
  }

  try {
    var mo = new MutationObserver(report);
    mo.observe(document.documentElement, { attributes: true, attributeFilter: ['class', 'style', 'data-theme'] });
    if (document.body) {
      mo.observe(document.body, { attributes: true, attributeFilter: ['class', 'style', 'data-theme'] });
    }
  } catch (e) {}
  try {
    var mq = window.matchMedia('(prefers-color-scheme: dark)');
    if (mq.addEventListener) { mq.addEventListener('change', report); }
    else if (mq.addListener) { mq.addListener(report); }
  } catch (e) {}
  window.addEventListener('resize', report);
  setInterval(report, 1200);
  report();
})();
"""

// MARK: - App delegate

final class AppDelegate: NSObject, NSApplicationDelegate, WKScriptMessageHandler {
    private var window: NSWindow!
    private var webView: WKWebView!
    private var titlebarStrip: TitlebarStripView!
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
        appMenu.addItem(withTitle: "打开日志目录",
                        action: #selector(openLogDirectory(_:)),
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

    /// Opens the log directory in Finder.
    ///
    /// Under the App Sandbox the logs live inside the app container
    /// (`~/Library/Containers/<bundle id>/Data/Library/Logs/co-shell-app`), which
    /// no other process can read. The app itself can, so this entry point is the
    /// only practical way to get at the hub log when something goes wrong.
    @objc private func openLogDirectory(_ sender: Any?) {
        NSWorkspace.shared.open(AppPaths.logDir)
    }

    private func buildWindow() {
        let rect = NSRect(x: 0, y: 0, width: 1280, height: 860)

        let configuration = WKWebViewConfiguration()
        let controller = WKUserContentController()
        controller.addUserScript(WKUserScript(source: themeProbeScript,
                                              injectionTime: .atDocumentEnd,
                                              forMainFrameOnly: true))
        controller.add(self, name: Titlebar.messageName)
        configuration.userContentController = controller

        // The shell owns the title-bar strip. With .fullSizeContentView the content
        // view spans the whole window and the transparent title bar lets it show
        // through, so the strip can be painted with the page's own colour instead
        // of a system material — and repainted whenever that colour changes.
        window = NSWindow(contentRect: rect,
                          styleMask: [.titled, .closable, .miniaturizable, .resizable, .fullSizeContentView],
                          backing: .buffered,
                          defer: false)
        window.title = AppPaths.displayName
        window.titleVisibility = .hidden
        window.titlebarAppearsTransparent = true
        window.backgroundColor = Titlebar.fallbackColor
        window.isOpaque = true

        // Content view layout: strip on top, page below it. The web view is inset
        // by the strip height, so page content is never covered — the shell only
        // paints the otherwise empty title-bar area.
        let container = NSView(frame: rect)
        webView = WKWebView(frame: .zero, configuration: configuration)
        webView.translatesAutoresizingMaskIntoConstraints = false
        container.addSubview(webView)

        let strip = TitlebarStripView(frame: .zero)
        strip.translatesAutoresizingMaskIntoConstraints = false
        // The strip paints itself (background + centred wordmark), so its colour
        // can follow the page theme.
        strip.setBarColor(Titlebar.fallbackColor)
        container.addSubview(strip)
        titlebarStrip = strip

        NSLayoutConstraint.activate([
            strip.leadingAnchor.constraint(equalTo: container.leadingAnchor),
            strip.trailingAnchor.constraint(equalTo: container.trailingAnchor),
            strip.topAnchor.constraint(equalTo: container.topAnchor),
            strip.heightAnchor.constraint(equalToConstant: Titlebar.height),

            webView.leadingAnchor.constraint(equalTo: container.leadingAnchor),
            webView.trailingAnchor.constraint(equalTo: container.trailingAnchor),
            webView.topAnchor.constraint(equalTo: strip.bottomAnchor),
            webView.bottomAnchor.constraint(equalTo: container.bottomAnchor),
        ])

        window.contentView = container
        window.center()
        window.makeKeyAndOrderFront(nil)
    }

    // MARK: - Page theme colour

    /// Repaints the strip with the colour the page reports for the area right
    /// below it, so the strip keeps matching the page when its theme changes.
    func userContentController(_ userContentController: WKUserContentController,
                               didReceive message: WKScriptMessage) {
        guard message.name == Titlebar.messageName,
              let css = message.body as? String,
              let color = Titlebar.color(fromCSS: css) else { return }
        titlebarStrip.setBarColor(color)
        window.backgroundColor = color
    }

    // MARK: - hub lifecycle

    private func startHub(webPort: UInt16) throws {
        let proc = Process()
        proc.executableURL = AppPaths.hubBinary
        proc.currentDirectoryURL = AppPaths.dataDir
        // --config/--registry are deliberately omitted, so hub resolves its own
        // defaults: ./hub-gateway.json and ./hub-agents.json relative to this
        // working directory (the app data dir), then ~/.co-shell/ as fallback.
        // --serve: the shell shows the hub Web UI in its own WKWebView window,
        // so hub must not open the system browser.
        proc.arguments = [
            "--web-addr", "127.0.0.1:\(webPort)",
            "--tcp-addr", "127.0.0.1:\(webPort &+ 1)",
            "--co-shell-path", AppPaths.coShellBinary.path,
            "--no-api-key",
            "--serve",
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
