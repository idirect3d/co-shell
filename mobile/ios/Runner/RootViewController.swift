import UIKit

/// Root navigation controller. Decides at launch whether to show the native
/// settings screen (not configured) or the hub web view (configured).
final class RootViewController: UINavigationController {

    override func viewDidLoad() {
        super.viewDidLoad()
        showInitialScreen()
    }

    private func showInitialScreen() {
        if SettingsStore.shared.isConfigured,
           let server = SettingsStore.shared.serverAddress,
           let key = SettingsStore.shared.accessKey {
            let web = WebViewController(serverAddress: server, accessKey: key)
            setViewControllers([web], animated: false)
        } else {
            let settings = SettingsViewController()
            settings.onSaved = { [weak self] in
                self?.showWebAfterConfigured()
            }
            setViewControllers([settings], animated: false)
        }
    }

    /// Called after the user saves a valid configuration from the initial
    /// settings screen: swap to the hub web view.
    private func showWebAfterConfigured() {
        guard let server = SettingsStore.shared.serverAddress,
              let key = SettingsStore.shared.accessKey else { return }
        let web = WebViewController(serverAddress: server, accessKey: key)
        setViewControllers([web], animated: true)
    }
}
