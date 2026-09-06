import UIKit

/// Native settings screen to configure the hub server address and access key.
/// Values are persisted in the Keychain via SettingsStore.
final class SettingsViewController: UIViewController {

    /// Called after a valid configuration is saved.
    var onSaved: (() -> Void)?

    private let serverField = UITextField()
    private let keyField = UITextField()
    private let saveButton = UIButton(type: .system)
    private let statusLabel = UILabel()

    override func viewDidLoad() {
        super.viewDidLoad()
        title = "设置"
        view.backgroundColor = .systemGroupedBackground

        // Show a close button only when presented modally (no back button).
        if navigationController?.viewControllers.first == self {
            navigationItem.leftBarButtonItem = UIBarButtonItem(
                title: "取消",
                style: .plain,
                target: self,
                action: #selector(cancelTapped)
            )
        }

        setupUI()
        loadCurrentValues()
    }

    private func setupUI() {
        let scroll = UIScrollView()
        scroll.translatesAutoresizingMaskIntoConstraints = false
        scroll.keyboardDismissMode = .interactive
        view.addSubview(scroll)

        let content = UIStackView()
        content.axis = .vertical
        content.spacing = 16
        content.translatesAutoresizingMaskIntoConstraints = false
        scroll.addSubview(content)

        NSLayoutConstraint.activate([
            scroll.topAnchor.constraint(equalTo: view.safeAreaLayoutGuide.topAnchor),
            scroll.bottomAnchor.constraint(equalTo: view.safeAreaLayoutGuide.bottomAnchor),
            scroll.leadingAnchor.constraint(equalTo: view.leadingAnchor),
            scroll.trailingAnchor.constraint(equalTo: view.trailingAnchor),
            content.topAnchor.constraint(equalTo: scroll.contentLayoutGuide.topAnchor, constant: 20),
            content.bottomAnchor.constraint(equalTo: scroll.contentLayoutGuide.bottomAnchor, constant: -20),
            content.leadingAnchor.constraint(equalTo: scroll.contentLayoutGuide.leadingAnchor, constant: 20),
            content.trailingAnchor.constraint(equalTo: scroll.contentLayoutGuide.trailingAnchor, constant: -20),
            content.widthAnchor.constraint(equalTo: scroll.frameLayoutGuide.widthAnchor, constant: -40),
            // FEATURE-487: keep the content at least as tall as the viewport so
            // the logo block can be centred in the empty area below the form.
            content.heightAnchor.constraint(greaterThanOrEqualTo: scroll.frameLayoutGuide.heightAnchor)
        ])

        let intro = UILabel()
        intro.text = "配置 co-shell hub 服务端地址与访问 KEY。访问 KEY 将自动注入浏览器，无需在网页中重复输入。"
        intro.numberOfLines = 0
        intro.font = .systemFont(ofSize: 14)
        intro.textColor = .secondaryLabel
        content.addArrangedSubview(intro)

        serverField.placeholder = "服务端地址，如 https://192.168.3.19:23311"
        serverField.borderStyle = .roundedRect
        serverField.autocapitalizationType = .none
        serverField.keyboardType = .URL
        serverField.autocorrectionType = .no
        content.addArrangedSubview(serverField)

        keyField.placeholder = "访问 KEY"
        keyField.borderStyle = .roundedRect
        keyField.autocapitalizationType = .none
        keyField.autocorrectionType = .no
        keyField.isSecureTextEntry = true
        content.addArrangedSubview(keyField)

        saveButton.setTitle("保存并连接", for: .normal)
        saveButton.titleLabel?.font = .systemFont(ofSize: 17, weight: .semibold)
        saveButton.backgroundColor = .systemBlue
        saveButton.setTitleColor(.white, for: .normal)
        saveButton.layer.cornerRadius = 10
        saveButton.heightAnchor.constraint(equalToConstant: 48).isActive = true
        saveButton.addTarget(self, action: #selector(saveTapped), for: .touchUpInside)
        content.addArrangedSubview(saveButton)

        statusLabel.text = ""
        statusLabel.numberOfLines = 0
        statusLabel.font = .systemFont(ofSize: 14)
        statusLabel.textColor = .systemRed
        content.addArrangedSubview(statusLabel)

        // FEATURE-487: a co-shell mosaic logo + version block centred in the
        // empty area below the form. Two flexible spacers (above and below)
        // distribute the leftover viewport space so the block stays centred
        // and the version label is never pushed off-screen.
        let topSpacer = UIView()
        topSpacer.setContentHuggingPriority(.defaultLow, for: .vertical)
        content.addArrangedSubview(topSpacer)

        let logo = CoShellLogoView(frame: CGRect(x: 0, y: 0, width: 72, height: 72))
        logo.cellColor = .systemTeal
        logo.translatesAutoresizingMaskIntoConstraints = false
        logo.widthAnchor.constraint(equalToConstant: 72).isActive = true
        logo.heightAnchor.constraint(equalToConstant: 72).isActive = true
        content.addArrangedSubview(logo)

        let nameLabel = UILabel()
        nameLabel.text = "co-shell mobile"
        nameLabel.font = .systemFont(ofSize: 16, weight: .semibold)
        nameLabel.textColor = .secondaryLabel
        nameLabel.textAlignment = .center
        content.addArrangedSubview(nameLabel)

        let verLabel = UILabel()
        verLabel.text = "v0.39.0"
        verLabel.font = .systemFont(ofSize: 12)
        verLabel.textColor = .tertiaryLabel
        verLabel.textAlignment = .center
        content.addArrangedSubview(verLabel)

        let bottomSpacer = UIView()
        bottomSpacer.setContentHuggingPriority(.defaultLow, for: .vertical)
        content.addArrangedSubview(bottomSpacer)
    }

    private func loadCurrentValues() {
        serverField.text = SettingsStore.shared.serverAddress
        keyField.text = SettingsStore.shared.accessKey
    }

    @objc private func cancelTapped() {
        dismiss(animated: true)
    }

    @objc private func saveTapped() {
        let server = serverField.text?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        let key = keyField.text?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""

        guard !server.isEmpty else {
            statusLabel.text = "请输入服务端地址"
            return
        }
        guard let url = URL(string: server), url.scheme != nil, url.host != nil else {
            statusLabel.text = "服务端地址格式无效，需包含协议，如 https://192.168.3.19:23311"
            return
        }
        guard !key.isEmpty else {
            statusLabel.text = "请输入访问 KEY"
            return
        }

        SettingsStore.shared.serverAddress = server
        SettingsStore.shared.accessKey = key

        if presentingViewController != nil {
            // Presented modally (e.g. as its own nav controller): dismiss.
            dismiss(animated: true) { [weak self] in
                self?.onSaved?()
            }
        } else if navigationController?.viewControllers.count ?? 0 > 1 {
            // Pushed onto an existing nav stack: pop back.
            navigationController?.popViewController(animated: true)
            onSaved?()
        } else {
            // Root of the nav controller (initial setup): let the owner react.
            onSaved?()
        }
    }
}
