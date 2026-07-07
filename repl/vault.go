package repl

import (
	"fmt"
	"strings"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/store"
)

// handleVaultCommand handles the :vault builtin command.
func (r *REPL) handleVaultCommand(args []string) (string, error) {
	vs := r.agent.VaultStore()
	if vs == nil {
		return "", fmt.Errorf("vault store not initialized")
	}

	if len(args) == 0 {
		return vs.Info(), nil
	}

	subcommand := args[0]

	switch subcommand {
	case "init":
		return handleVaultInit(vs)
	case "unlock":
		return handleVaultUnlock(vs, args[1:])
	case "lock":
		vs.Lock()
		return "vault locked", nil
	case "list":
		return handleVaultList(vs)
	case "show":
		return handleVaultShow(vs, args[1:])
	case "add":
		return handleVaultAdd(vs, args[1:])
	case "remove":
		return handleVaultRemove(vs, args[1:])
	default:
		return "", fmt.Errorf("unknown vault subcommand: %s", subcommand)
	}
}

func handleVaultInit(vs *store.VaultStore) (string, error) {
	if vs.IsInitialized() {
		return "", fmt.Errorf("vault is already initialized")
	}
	fmt.Println(i18n.T(i18n.KeyVaultInitPrompt))
	fmt.Print("  master password: ")
	password, err := readPassword()
	if err != nil {
		return "", fmt.Errorf("cannot read password: %w", err)
	}
	password = strings.TrimSpace(password)
	if password == "" {
		return "", fmt.Errorf("password cannot be empty")
	}
	fmt.Print("  confirm password: ")
	confirm, err := readPassword()
	if err != nil {
		return "", fmt.Errorf("cannot read password: %w", err)
	}
	confirm = strings.TrimSpace(confirm)
	if password != confirm {
		return "", fmt.Errorf("passwords do not match")
	}
	fmt.Print("  algorithm (aes/sm4, default=sm4): ")
	algo, _ := readLine(nil)
	algo = strings.TrimSpace(strings.ToLower(algo))
	if algo == "" {
		algo = "sm4"
	}
	if algo != "aes" && algo != "sm4" {
		return "", fmt.Errorf("unsupported algorithm: %s (use aes or sm4)", algo)
	}
	if err := vs.Init(password, algo); err != nil {
		return "", fmt.Errorf("cannot initialize vault: %w", err)
	}
	return fmt.Sprintf("vault initialized (algorithm: %s)", algo), nil
}

func handleVaultUnlock(vs *store.VaultStore, args []string) (string, error) {
	if vs.IsUnlocked() {
		return "vault is already unlocked", nil
	}
	var password string
	if len(args) > 0 {
		password = strings.Join(args, " ")
	} else {
		fmt.Print("  master password: ")
		var err error
		password, err = readPassword()
		if err != nil {
			return "", fmt.Errorf("cannot read password: %w", err)
		}
		password = strings.TrimSpace(password)
	}
	if password == "" {
		return "", fmt.Errorf("password cannot be empty")
	}
	if err := vs.Unlock(password); err != nil {
		return "", fmt.Errorf("unlock failed: %w", err)
	}
	return "vault unlocked", nil
}

func handleVaultList(vs *store.VaultStore) (string, error) {
	if !vs.IsUnlocked() {
		return "", fmt.Errorf("vault is locked, use :vault unlock first")
	}
	names, err := vs.List()
	if err != nil {
		return "", fmt.Errorf("cannot list vault: %w", err)
	}
	if len(names) == 0 {
		return "vault: no entries", nil
	}
	var b strings.Builder
	b.WriteString("vault entries:\n")
	for _, name := range names {
		b.WriteString(fmt.Sprintf("  - %s\n", name))
	}
	b.WriteString(fmt.Sprintf("\ntotal: %d entries", len(names)))
	return b.String(), nil
}

func handleVaultShow(vs *store.VaultStore, args []string) (string, error) {
	if !vs.IsUnlocked() {
		return "", fmt.Errorf("vault is locked, use :vault unlock first")
	}
	if len(args) == 0 {
		return "", fmt.Errorf("usage: :vault show <name>")
	}
	name := args[0]
	entry, err := vs.Get(name)
	if err != nil {
		return "", fmt.Errorf("cannot get entry %q: %w", name, err)
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("name:     %s\n", entry.Name))
	if len(entry.Tags) > 0 {
		b.WriteString("tags:\n")
		for tag, val := range entry.Tags {
			b.WriteString(fmt.Sprintf("  %s: %s\n", tag, val))
		}
	}
	if entry.Notes != "" {
		b.WriteString(fmt.Sprintf("notes:    %s\n", entry.Notes))
	}
	b.WriteString(fmt.Sprintf("algo:     %s\n", entry.Algorithm))
	return b.String(), nil
}

func handleVaultAdd(vs *store.VaultStore, args []string) (string, error) {
	if !vs.IsUnlocked() {
		return "", fmt.Errorf("vault is locked, use :vault unlock first")
	}
	if len(args) == 0 {
		return "", fmt.Errorf("usage: :vault add <name>")
	}
	name := args[0]

	fmt.Println("  请输入标签值对（格式: tag=value），每行一个，空行结束")
	fmt.Println("  例如: user=myuser, pwd=mypass, key=xxx, token=xxx, email=xxx, ip_addr=1.2.3.4")

	tags := make(map[string]string)
	for {
		fmt.Printf("  tag=value: ")
		line, err := readPassword()
		if err != nil {
			return "", fmt.Errorf("failed to read input: %w", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			fmt.Println("  格式错误，应为 tag=value")
			continue
		}
		tagName := strings.TrimSpace(parts[0])
		tagValue := strings.TrimSpace(parts[1])
		if tagName == "" || tagValue == "" {
			fmt.Println("  tag 和 value 都不能为空")
			continue
		}
		tags[tagName] = tagValue
	}
	if len(tags) == 0 {
		return "", fmt.Errorf("at least one tag is required")
	}

	entry := &store.VaultEntry{Name: name, Tags: tags}
	if err := vs.Put(entry); err != nil {
		return "", fmt.Errorf("cannot save entry: %w", err)
	}
	return fmt.Sprintf("entry %q added with tags: %s", name, strings.Join(mapKeys(tags), ", ")), nil
}

func mapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func handleVaultRemove(vs *store.VaultStore, args []string) (string, error) {
	if !vs.IsUnlocked() {
		return "", fmt.Errorf("vault is locked, use :vault unlock first")
	}
	if len(args) == 0 {
		return "", fmt.Errorf("usage: :vault remove <name>")
	}
	name := args[0]
	if err := vs.Delete(name); err != nil {
		return "", fmt.Errorf("cannot remove entry %q: %w", name, err)
	}
	return fmt.Sprintf("entry %q removed", name), nil
}

func readPassword() (string, error) {
	return readLine(nil)
}

func (r *REPL) defaultIO() *REPL {
	return r
}

func readLine(_ interface{}) (string, error) {
	var s string
	_, err := fmt.Scanln(&s)
	return s, err
}
