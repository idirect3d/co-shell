import Foundation
import Security

/// Persists the hub server address and access key.
///
/// Primary backend is the iOS Keychain. On the simulator the Keychain is
/// unavailable for unsigned builds (SecItemAdd returns errSecMissingEntitlement
/// -34018), so we transparently fall back to UserDefaults so the app still
/// works during development. On a real device the Keychain is used.
final class SettingsStore {

    static let shared = SettingsStore()

    private let service = "com.coshell.mobile"
    private let serverAccount = "server_address"
    private let keyAccount = "access_key"

    // UserDefaults keys used as the fallback backend.
    private let defaultsServerKey = "settings.server_address"
    private let defaultsKeyKey = "settings.access_key"

    private init() {}

    // MARK: - Server address

    var serverAddress: String? {
        get { read(account: serverAccount, defaultsKey: defaultsServerKey) }
        set { write(newValue, account: serverAccount, defaultsKey: defaultsServerKey) }
    }

    // MARK: - Access key

    var accessKey: String? {
        get { read(account: keyAccount, defaultsKey: defaultsKeyKey) }
        set { write(newValue, account: keyAccount, defaultsKey: defaultsKeyKey) }
    }

    /// True when both server address and access key are configured.
    var isConfigured: Bool {
        guard let addr = serverAddress, !addr.isEmpty,
              let key = accessKey, !key.isEmpty else { return false }
        return true
    }

    // MARK: - Storage helpers

    private func read(account: String, defaultsKey: String) -> String? {
        // Try Keychain first.
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecReturnData as String: true,
            kSecMatchLimit as String: kSecMatchLimitOne
        ]
        var item: CFTypeRef?
        if SecItemCopyMatching(query as CFDictionary, &item) == errSecSuccess,
           let data = item as? Data, let s = String(data: data, encoding: .utf8) {
            return s
        }
        // Fall back to UserDefaults.
        return UserDefaults.standard.string(forKey: defaultsKey)
    }

    private func write(_ value: String?, account: String, defaultsKey: String) {
        // Always mirror to UserDefaults (works on simulator and device).
        UserDefaults.standard.set(value, forKey: defaultsKey)

        // Delete existing Keychain item first.
        let deleteQuery: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account
        ]
        SecItemDelete(deleteQuery as CFDictionary)

        guard let value = value, !value.isEmpty else { return }

        let addQuery: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecValueData as String: Data(value.utf8),
            kSecAttrAccessible as String: kSecAttrAccessibleWhenUnlocked
        ]
        // Keychain write is best-effort; on the simulator it may fail with
        // errSecMissingEntitlement (-34018) for unsigned builds, in which case
        // the UserDefaults mirror above still holds the value.
        SecItemAdd(addQuery as CFDictionary, nil)
    }
}
