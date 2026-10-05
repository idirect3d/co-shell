import UIKit
import WebKit

/// Hosts the hub web UI in a WKWebView. Injects the access_key cookie before
/// loading the configured server address, and tolerates self-signed TLS certs.
final class WebViewController: UIViewController {

    private var webView: WKWebView!
    private let serverAddress: String
    private let accessKey: String

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
        let loadingBackground = UIColor.black
        view.backgroundColor = loadingBackground

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

        let config = WKWebViewConfiguration()
        config.websiteDataStore = .default()
        config.allowsInlineMediaPlayback = true

        webView = WKWebView(frame: .zero, configuration: config)
        webView.isOpaque = false
        webView.backgroundColor = loadingBackground
        webView.scrollView.backgroundColor = loadingBackground
        webView.navigationDelegate = self
        webView.uiDelegate = self
        webView.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(webView)

        NSLayoutConstraint.activate([
            webView.topAnchor.constraint(equalTo: view.topAnchor),
            webView.bottomAnchor.constraint(equalTo: view.bottomAnchor),
            webView.leadingAnchor.constraint(equalTo: view.leadingAnchor),
            webView.trailingAnchor.constraint(equalTo: view.trailingAnchor)
        ])

        loadHub()
    }

    override func viewWillAppear(_ animated: Bool) {
        super.viewWillAppear(animated)
        // Reload when returning from the settings screen so a changed
        // server address / key takes effect.
        if didReturnFromSettings {
            didReturnFromSettings = false
            loadHub()
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

    private func loadHub() {
        guard let url = URL(string: serverAddress) else {
            showError("服务端地址无效")
            return
        }
        // Inject access_key cookie for the hub host so the user need not type it.
        let cookie = HTTPCookie(properties: [
            .domain: url.host ?? "",
            .path: "/",
            .name: "access_key",
            .value: accessKey,
            .secure: url.scheme == "https",
            .expires: Date(timeIntervalSinceNow: 60 * 60 * 24 * 365)
        ])
        if let cookie = cookie {
            webView.configuration.websiteDataStore.httpCookieStore.setCookie(cookie) { [weak self] in
                self?.webView.load(URLRequest(url: url))
            }
        } else {
            webView.load(URLRequest(url: url))
        }
    }

    @objc private func refreshTapped() {
        webView.reload()
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

    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
        showError(error.localizedDescription)
    }

    func webView(
        _ webView: WKWebView,
        didFailProvisionalNavigation navigation: WKNavigation!,
        withError error: Error
    ) {
        // Ignore cancellation errors (e.g. when reloading).
        if (error as NSError).code == NSURLErrorCancelled { return }
        showError(error.localizedDescription)
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
