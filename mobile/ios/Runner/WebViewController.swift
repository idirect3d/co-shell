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
        view.backgroundColor = .systemBackground

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

    /// Builds the navigation title: a small co-shell mosaic logo followed by
    /// the "co-shell" wordmark.
    private func makeTitleView() -> UIView {
        let logo = CoShellLogoView(frame: CGRect(x: 0, y: 0, width: 22, height: 22))
        logo.cellColor = .systemTeal

        let label = UILabel()
        label.text = "co-shell"
        label.font = .systemFont(ofSize: 17, weight: .semibold)
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
