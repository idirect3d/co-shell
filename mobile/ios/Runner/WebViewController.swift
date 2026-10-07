import OSLog
import UIKit
import WebKit

/// Hosts the hub web UI in a WKWebView. Injects the access_key cookie before
/// loading the configured server address, and tolerates self-signed TLS certs.
final class WebViewController: UIViewController {

    /// FIX-619: diagnostics. The load lifecycle (start / finish / failure) is
    /// logged with the URL, the error domain+code and the elapsed time, so a
    /// failed refresh can be classified afterwards: "the request never left the
    /// device" (no matching entry in the hub request log) versus "it reached the
    /// hub and timed out".
    private static let log = Logger(subsystem: "com.coshell.mobile", category: "webview")

    private var webView: WKWebView!
    private let serverAddress: String
    private let accessKey: String

    /// FIX-619: when the current load started, used to report its duration.
    private var loadStartedAt: Date?

    /// FIX-619: how many automatic retries a failed load gets, and the backoff
    /// between attempts. Once they are exhausted the web view is rebuilt once.
    private static let retryDelays: [TimeInterval] = [1, 3, 5]

    /// FIX-619: request timeout for a (re)load. The system default of 60s keeps
    /// the UI hanging on a connection the mobile network already dropped; a
    /// short timeout lets the retry / rebuild path actually run.
    private static let refreshTimeout: TimeInterval = 12

    /// FIX-619: the (dark) background used while loading — also given to a web
    /// view rebuilt by the refresh path.
    private static let loadingBackground = UIColor.black

    /// FIX-619: retry bookkeeping for the load currently in flight.
    private var retryIndex = 0
    private var didRebuildForCurrentLoad = false
    private var lastLoadError: Error?

    init(serverAddress: String, accessKey: String) {
        self.serverAddress = serverAddress
        self.accessKey = accessKey
        super.init(nibName: nil, bundle: nil)
    }

    required init?(coder: NSCoder) {
        fatalError("init(coder:) has not been implemented")
    }

    override func viewDidLoad() {
        super.viewDidLoad()
        // FIX-585: the launch screen is dark, so keep the web view loading phase
        // dark as well — WKWebView defaults to an opaque white layer, which used
        // to flash white between the launch screen and the hub page.
        view.backgroundColor = Self.loadingBackground

        // Left: a refresh button to reload the hub page.
        navigationItem.leftBarButtonItem = UIBarButtonItem(
            image: UIImage(systemName: "arrow.clockwise"),
            style: .plain,
            target: self,
            action: #selector(refreshTapped)
        )
        // Right: open the native settings screen.
        navigationItem.rightBarButtonItem = UIBarButtonItem(
            title: "设置",
            style: .plain,
            target: self,
            action: #selector(openSettings)
        )
        // Title: co-shell mosaic logo + "co-shell" text.
        navigationItem.titleView = makeTitleView()

        let wv = makeWebView()
        webView = wv
        view.addSubview(wv)

        NSLayoutConstraint.activate([
            wv.topAnchor.constraint(equalTo: view.topAnchor),
            wv.bottomAnchor.constraint(equalTo: view.bottomAnchor),
            wv.leadingAnchor.constraint(equalTo: view.leadingAnchor),
            wv.trailingAnchor.constraint(equalTo: view.trailingAnchor)
        ])

        loadHub()
    }

    override func viewWillAppear(_ animated: Bool) {
        super.viewWillAppear(animated)
        // Reload when returning from the settings screen so a changed
        // server address / key takes effect.
        if didReturnFromSettings {
            didReturnFromSettings = false
            startFreshLoad()
        }
    }

    private var didReturnFromSettings = false

    /// FIX-608: destination of the download currently in flight, kept so the
    /// share sheet can be shown once the download finishes.
    private var pendingDownloadURL: URL?

    /// Builds the navigation title: a small co-shell mosaic logo followed by
    /// the "co-shell" wordmark.
    private func makeTitleView() -> UIView {
        let logo = CoShellLogoView(frame: CGRect(x: 0, y: 0, width: 33, height: 33))
        logo.cellColor = .systemTeal

        let label = UILabel()
        label.text = "co-shell"
        // Retro low-resolution terminal look: Menlo monospace font.
        label.font = UIFont(name: "Menlo-Bold", size: 17) ?? .systemFont(ofSize: 17, weight: .semibold)
        label.textColor = .label

        let stack = UIStackView(arrangedSubviews: [logo, label])
        stack.axis = .horizontal
        stack.spacing = 6
        stack.alignment = .center
        return stack
    }

    /// loadHub loads the configured hub address, injecting the access_key cookie
    /// first. FIX-619: the cache policy and timeout are parameters so the refresh
    /// path can bypass cached responses and fail fast instead of hanging on a
    /// connection the network already dropped.
    private func loadHub(cachePolicy: URLRequest.CachePolicy = .useProtocolCachePolicy,
                         timeout: TimeInterval = WebViewController.refreshTimeout) {
        guard let url = URL(string: serverAddress) else {
            showError("服务端地址无效")
            return
        }
        loadStartedAt = Date()
        Self.log.info("load start url=\(url.absoluteString, privacy: .public) timeout=\(timeout, privacy: .public)s")
        // Inject access_key cookie for the hub host so the user need not type it.
        let cookie = HTTPCookie(properties: [
            .domain: url.host ?? "",
            .path: "/",
            .name: "access_key",
            .value: accessKey,
            .secure: url.scheme == "https",
            .expires: Date(timeIntervalSinceNow: 60 * 60 * 24 * 365)
        ])
        let request = URLRequest(url: url, cachePolicy: cachePolicy, timeoutInterval: timeout)
        if let cookie = cookie {
            webView.configuration.websiteDataStore.httpCookieStore.setCookie(cookie) { [weak self] in
                self?.webView.load(request)
            }
        } else {
            webView.load(request)
        }
    }

    /// makeWebView builds a configured web view: opaque off, dark background,
    /// delegates wired. FIX-619: shared by the initial load and by the rebuild
    /// done when reloads keep failing.
    private func makeWebView() -> WKWebView {
        let config = WKWebViewConfiguration()
        config.websiteDataStore = .default()
        config.allowsInlineMediaPlayback = true

        let wv = WKWebView(frame: .zero, configuration: config)
        wv.isOpaque = false
        wv.backgroundColor = Self.loadingBackground
        wv.scrollView.backgroundColor = Self.loadingBackground
        wv.navigationDelegate = self
        wv.uiDelegate = self
        wv.translatesAutoresizingMaskIntoConstraints = false
        return wv
    }

    /// rebuildWebView replaces the web view with a fresh instance. It is the last
    /// resort of the refresh path: a new WKWebView gets a new network stack, which
    /// is exactly what "kill the app and open it again" does today — done in
    /// process.
    private func rebuildWebView() {
        webView?.stopLoading()
        webView?.navigationDelegate = nil
        webView?.uiDelegate = nil
        webView?.removeFromSuperview()

        let wv = makeWebView()
        webView = wv
        view.addSubview(wv)
        NSLayoutConstraint.activate([
            wv.topAnchor.constraint(equalTo: view.topAnchor),
            wv.bottomAnchor.constraint(equalTo: view.bottomAnchor),
            wv.leadingAnchor.constraint(equalTo: view.leadingAnchor),
            wv.trailingAnchor.constraint(equalTo: view.trailingAnchor)
        ])
        loadHub(cachePolicy: .reloadIgnoringLocalCacheData)
    }

    /// startFreshLoad is the manual refresh (FIX-619): reset the retry budget,
    /// drop caches and load again from scratch instead of merely reloading the
    /// document on a possibly poisoned web view.
    private func startFreshLoad() {
        retryIndex = 0
        didRebuildForCurrentLoad = false
        lastLoadError = nil
        reloadNow()
    }

    /// reloadNow stops whatever is in flight, drops the web view's cached
    /// responses (cookies are kept — the access_key must survive) and loads again
    /// while bypassing caches.
    private func reloadNow() {
        webView?.stopLoading()
        Self.clearWebCaches { [weak self] in
            self?.loadHub(cachePolicy: .reloadIgnoringLocalCacheData)
        }
    }

    /// clearWebCaches drops the web view's memory and disk caches only; cookies,
    /// local storage and the access_key injected for the hub are left intact.
    private static func clearWebCaches(completion: @escaping () -> Void) {
        URLCache.shared.removeAllCachedResponses()
        let types: Set<String> = [WKWebsiteDataTypeMemoryCache, WKWebsiteDataTypeDiskCache]
        WKWebsiteDataStore.default().removeData(ofTypes: types, modifiedSince: .distantPast) {
            DispatchQueue.main.async { completion() }
        }
    }

    /// retryLoad re-issues a failed load: a few attempts with backoff, then one
    /// web-view rebuild, and only then the error alert. The failure behind FIX-619
    /// (a mobile network that silently dropped the connection while the app was
    /// suspended) is transient, so retrying heals it without killing the app.
    private func retryLoad(_ error: Error) {
        lastLoadError = error
        if retryIndex < Self.retryDelays.count {
            let delay = Self.retryDelays[retryIndex]
            retryIndex += 1
            Self.log.info("load retry \(self.retryIndex, privacy: .public)/\(Self.retryDelays.count, privacy: .public) in \(delay, privacy: .public)s")
            DispatchQueue.main.asyncAfter(deadline: .now() + delay) { [weak self] in
                self?.reloadNow()
            }
            return
        }
        if !didRebuildForCurrentLoad {
            didRebuildForCurrentLoad = true
            // No further retries after the rebuild: report the failure instead of
            // looping forever.
            retryIndex = Self.retryDelays.count
            Self.log.error("load retries exhausted; rebuilding the web view")
            rebuildWebView()
            return
        }
        showError(lastLoadError?.localizedDescription ?? "无法连接服务器")
    }

    @objc private func refreshTapped() {
        startFreshLoad()
    }

    @objc private func openSettings() {
        let settings = SettingsViewController()
        settings.onSaved = { [weak self] in
            self?.didReturnFromSettings = true
        }
        navigationController?.pushViewController(settings, animated: true)
    }

    private func showError(_ message: String) {
        let alert = UIAlertController(title: "错误", message: message, preferredStyle: .alert)
        alert.addAction(UIAlertAction(title: "确定", style: .default))
        present(alert, animated: true)
    }
}

// MARK: - WKNavigationDelegate

extension WebViewController: WKNavigationDelegate {

    func webView(
        _ webView: WKWebView,
        didReceive challenge: URLAuthenticationChallenge,
        completionHandler: @escaping (URLSession.AuthChallengeDisposition, URLCredential?) -> Void
    ) {
        // Accept self-signed certificates (trust-on-first-use for local hub).
        guard let serverTrust = challenge.protectionSpace.serverTrust else {
            completionHandler(.performDefaultHandling, nil)
            return
        }
        let credential = URLCredential(trust: serverTrust)
        completionHandler(.useCredential, credential)
    }

    func webView(
        _ webView: WKWebView,
        decidePolicyFor navigationResponse: WKNavigationResponse,
        decisionHandler: @escaping (WKNavigationResponsePolicy) -> Void
    ) {
        // FIX-608: WKWebView cannot render a download response
        // (Content-Disposition: attachment). Without this branch WebKit simply
        // cancels the navigation, and since the request already replaced the
        // SPA document, the mobile UI was left blank and unresponsive. Hand
        // such responses to WKDownload so the file is saved and the page stays.
        if !navigationResponse.canShowMIMEType || Self.isAttachment(navigationResponse.response) {
            decisionHandler(.download)
            return
        }
        decisionHandler(.allow)
    }

    /// isAttachment reports whether a response is served as a file download.
    private static func isAttachment(_ response: URLResponse) -> Bool {
        guard let http = response as? HTTPURLResponse,
              let disposition = http.value(forHTTPHeaderField: "Content-Disposition")
        else { return false }
        return disposition.lowercased().hasPrefix("attachment")
    }

    func webView(
        _ webView: WKWebView,
        navigationResponse: WKNavigationResponse,
        didBecome download: WKDownload
    ) {
        download.delegate = self
    }

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        // FIX-619: a successful load is the healthy reference point for the
        // duration comparison when a later reload hangs.
        Self.log.info("load finish url=\(webView.url?.absoluteString ?? "-", privacy: .public) elapsed=\(Self.elapsedMs(since: self.loadStartedAt), privacy: .public)ms")
        loadStartedAt = nil
        // FIX-619: a completed load clears the retry budget for the next one.
        retryIndex = 0
        didRebuildForCurrentLoad = false
        lastLoadError = nil
    }

    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
        logLoadFailure(error, phase: "navigation")
        // FIX-619: retry (then rebuild) instead of alerting right away.
        retryLoad(error)
    }

    func webView(
        _ webView: WKWebView,
        didFailProvisionalNavigation navigation: WKNavigation!,
        withError error: Error
    ) {
        // FIX-619: log every failure, including cancellations (a reload aborts
        // the load in flight), so the sequence stays readable.
        logLoadFailure(error, phase: "provisional")
        // Ignore cancellation errors (e.g. when reloading).
        if (error as NSError).code == NSURLErrorCancelled { return }
        // FIX-619: retry (then rebuild) instead of alerting right away.
        retryLoad(error)
    }

    /// logLoadFailure records a failed load with the fields needed to tell a
    /// client-side stall (never left the device) from a server-side timeout.
    private func logLoadFailure(_ error: Error, phase: String) {
        let ns = error as NSError
        Self.log.error("load failed phase=\(phase, privacy: .public) url=\(self.webView?.url?.absoluteString ?? "-", privacy: .public) domain=\(ns.domain, privacy: .public) code=\(ns.code, privacy: .public) elapsed=\(Self.elapsedMs(since: self.loadStartedAt), privacy: .public)ms desc=\(ns.localizedDescription, privacy: .public)")
    }

    /// elapsedMs reports how long the current load has been running; -1 when no
    /// load is in flight.
    private static func elapsedMs(since start: Date?) -> Int {
        guard let start = start else { return -1 }
        return Int(Date().timeIntervalSince(start) * 1000)
    }
}

// MARK: - WKUIDelegate

extension WebViewController: WKUIDelegate {

    func webView(
        _ webView: WKWebView,
        createWebViewWith configuration: WKWebViewConfiguration,
        for navigationAction: WKNavigationAction,
        windowFeatures: WKWindowFeatures
    ) -> WKWebView? {
        // Open target=_blank links in the same web view.
        if navigationAction.targetFrame == nil {
            webView.load(navigationAction.request)
        }
        return nil
    }
}

// MARK: - WKDownloadDelegate (FIX-608)

extension WebViewController: WKDownloadDelegate {

    func download(
        _ download: WKDownload,
        decideDestinationUsing response: URLResponse,
        suggestedFilename: String,
        completionHandler: @escaping (URL?) -> Void
    ) {
        let dir = Self.downloadDirectory()
        do {
            try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        } catch {
            completionHandler(nil)
            return
        }
        let target = Self.destination(in: dir, suggestedFilename: suggestedFilename)
        pendingDownloadURL = target
        completionHandler(target)
    }

    func downloadDidFinish(_ download: WKDownload) {
        guard let url = pendingDownloadURL else { return }
        pendingDownloadURL = nil
        // Let the user keep the file (save into the Files app) or share it.
        let activity = UIActivityViewController(activityItems: [url], applicationActivities: nil)
        if let popover = activity.popoverPresentationController {
            popover.sourceView = view
            popover.sourceRect = view.bounds
        }
        present(activity, animated: true)
    }

    func download(_ download: WKDownload, didFailWithError error: Error, resumeData: Data?) {
        pendingDownloadURL = nil
        // A download the user cancelled is not an error worth an alert.
        if (error as NSError).code == NSURLErrorCancelled { return }
        showError(error.localizedDescription)
    }

    /// downloadDirectory is the sandbox folder downloaded workspace files land in.
    private static func downloadDirectory() -> URL {
        let documents = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0]
        return documents.appendingPathComponent("download", isDirectory: true)
    }

    /// destination sanitizes the suggested file name and makes it unique inside
    /// `dir`, so a repeated download never overwrites an earlier file.
    private static func destination(in dir: URL, suggestedFilename: String) -> URL {
        var name = suggestedFilename.isEmpty ? "download" : suggestedFilename
        // The name comes from the server: keep it a plain file name inside dir.
        name = name.replacingOccurrences(of: "/", with: "_").replacingOccurrences(of: "\\", with: "_")
        while name.hasPrefix(".") { name.removeFirst() }
        if name.isEmpty { name = "download" }

        let ext = (name as NSString).pathExtension
        let base = (name as NSString).deletingPathExtension
        var candidate = dir.appendingPathComponent(name)
        var index = 1
        while FileManager.default.fileExists(atPath: candidate.path) {
            let suffix = ext.isEmpty ? "" : ".\(ext)"
            candidate = dir.appendingPathComponent("\(base)-\(index)\(suffix)")
            index += 1
        }
        return candidate
    }
}
