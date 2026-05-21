// Author: L.Shuang
// Created: 2026-05-21
// Last Modified: 2026-05-21
//
// # MIT License
//
// # Copyright (c) 2026 L.Shuang
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/store"
)

// HandleDB handles the .db built-in command.
// Usage:
//
//	.db              - Show current database configuration
//	.db config       - Launch the database configuration wizard
//	.db init         - Initialize PostgreSQL database (drop and recreate all tables)
//	.db migrate      - Migrate data from local bbolt to PostgreSQL
//	.db backup       - Backup all PostgreSQL tables to CSV files
//	.db restore      - Restore PostgreSQL data from a backup
//	.db <subkey> <value> - Set a specific DB parameter (same as .set db <subkey> <value>)
func (h *SettingsHandler) HandleDB(args []string) (string, error) {
	if len(args) == 0 {
		// .db -> show current config and usage
		return h.showDBConfig()
	}
	switch args[0] {
	case "config":
		// .db config -> launch configuration wizard
		return h.dbConfigWizard()
	case "init":
		// .db init -> initialize PostgreSQL database
		return h.dbInit()
	case "migrate":
		// .db migrate -> migrate data from bbolt to PostgreSQL
		return h.dbMigrate()
	case "backup":
		// .db backup -> backup all tables to CSV
		return h.dbBackup()
	case "restore":
		// .db restore -> restore from backup
		return h.dbRestore()
	default:
		// .db <subkey> <value> -> delegate to handleDBSubCommand
		return h.handleDBSubCommand(args)
	}
}

// showDBConfig displays the current DB configuration and usage instructions.
// Format follows the same pattern as showSettingsHelp: name: value col3
func (h *SettingsHandler) showDBConfig() (string, error) {
	enabledStatus := i18n.T(i18n.KeyOff)
	if h.cfg.DB.Enabled {
		enabledStatus = i18n.T(i18n.KeyOn)
	}
	var sb strings.Builder
	sb.WriteString(i18n.T(i18n.KeyDBConfigLabel) + ":\n")
	// Format: name: value col3 (name uses config key ID, col3 uses translated label)
	sb.WriteString(fmt.Sprintf("  %-20s %-20s %s\n", "enabled:", enabledStatus, i18n.T(i18n.KeyDBEnabledLabel)))
	sb.WriteString(fmt.Sprintf("  %-20s %-20s %s\n", "host:", h.cfg.DB.Host, i18n.T(i18n.KeyDBHostLabel)))
	sb.WriteString(fmt.Sprintf("  %-20s %-20d %s\n", "port:", h.cfg.DB.Port, i18n.T(i18n.KeyDBPortLabel)))
	sb.WriteString(fmt.Sprintf("  %-20s %-20s %s\n", "name:", h.cfg.DB.DBName, i18n.T(i18n.KeyDBNameLabel)))
	sb.WriteString(fmt.Sprintf("  %-20s %-20s %s\n", "schema:", h.cfg.DB.Schema, i18n.T(i18n.KeyDBSchemaLabel)))
	sb.WriteString(fmt.Sprintf("  %-20s %-20s %s\n", "user:", h.cfg.DB.User, i18n.T(i18n.KeyDBUserLabel)))
	sb.WriteString(fmt.Sprintf("  %-20s %-20s %s\n", "password:", "****", i18n.T(i18n.KeyDBPasswordLabel)))
	sb.WriteString("\n.set db <key> <value> - " + i18n.T(i18n.KeyDBSubCmdDesc) + "\n")
	sb.WriteString(".db config - " + i18n.T(i18n.KeyDBConfigLabel) + "\n")
	sb.WriteString(".db init - " + i18n.T(i18n.KeyDBInitDesc) + "\n")
	sb.WriteString(".db migrate - " + i18n.T(i18n.KeyDBMigrateDesc) + "\n")
	sb.WriteString(".db backup - " + i18n.T(i18n.KeyDBBackupTitle) + "\n")
	sb.WriteString(".db restore - " + i18n.T(i18n.KeyDBRestoreTitle) + "\n")
	return sb.String(), nil
}

// handleDBSubCommand handles the .set db <subkey> <value> sub-command.
func (h *SettingsHandler) handleDBSubCommand(args []string) (string, error) {
	if len(args) == 0 {
		return h.showDBConfig()
	}

	subkey := args[0]
	switch subkey {
	case "enabled":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOff)
			if h.cfg.DB.Enabled {
				status = i18n.T(i18n.KeyOn)
			}
			return fmt.Sprintf("数据库连接: %s", status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.DB.Enabled = true
		case "off", "0", "false", "no":
			h.cfg.DB.Enabled = false
		default:
			return "", fmt.Errorf("usage: .set db enabled on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.DB.Enabled {
			status = i18n.T(i18n.KeyOff)
			log.Info("DB enabled set to %s", status)
			return fmt.Sprintf("✅ 数据库连接已设置为: %s", status), nil
		}

		// When enabling DB, immediately test the connection with current parameters
		log.Info("DB enabled set to %s", status)
		fmt.Println("\n🔌 正在测试数据库连接...")
		pgStore, err := store.NewPGStore(h.cfg.DB)
		if err != nil {
			fmt.Printf("❌ 数据库连接失败: %v\n", err)
			fmt.Print("是否启动数据库配置向导进行参数配置? (y/n, 默认: y): ")
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				switch strings.ToLower(line) {
				case "n", "no", "off", "0", "false":
					return "✅ 数据库连接已启用，但连接测试未通过，请检查配置后重试", nil
				default:
					return h.dbConfigWizard()
				}
			}
			return "✅ 数据库连接已启用，但连接测试未通过", nil
		}
		pgStore.Close()
		fmt.Println("✅ 数据库连接成功!")
		return fmt.Sprintf("✅ 数据库连接已设置为: %s", status), nil

	case "host":
		if len(args) < 2 {
			return fmt.Sprintf("数据库主机: %s", h.cfg.DB.Host), nil
		}
		h.cfg.DB.Host = args[1]
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("DB host set to %s", args[1])
		return fmt.Sprintf("✅ 数据库主机已设置为: %s", args[1]), nil

	case "port":
		if len(args) < 2 {
			return fmt.Sprintf("数据库端口: %d", h.cfg.DB.Port), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("无效的端口号: %s", args[1])
		}
		if n < 1 || n > 65535 {
			return "", fmt.Errorf("端口号必须在 1 ~ 65535 之间")
		}
		h.cfg.DB.Port = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("DB port set to %d", n)
		return fmt.Sprintf("✅ 数据库端口已设置为: %d", n), nil

	case "name":
		if len(args) < 2 {
			return fmt.Sprintf("数据库名称: %s", h.cfg.DB.DBName), nil
		}
		h.cfg.DB.DBName = args[1]
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("DB name set to %s", args[1])
		return fmt.Sprintf("✅ 数据库名称已设置为: %s", args[1]), nil

	case "schema":
		if len(args) < 2 {
			return fmt.Sprintf("数据库 Schema: %s", h.cfg.DB.Schema), nil
		}
		h.cfg.DB.Schema = args[1]
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("DB schema set to %s", args[1])
		return fmt.Sprintf("✅ 数据库 Schema 已设置为: %s", args[1]), nil

	case "user":
		if len(args) < 2 {
			return fmt.Sprintf("数据库用户: %s", h.cfg.DB.User), nil
		}
		h.cfg.DB.User = args[1]
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("DB user set to %s", args[1])
		return fmt.Sprintf("✅ 数据库用户已设置为: %s", args[1]), nil

	case "password":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: .set db password <password>")
		}
		h.cfg.DB.Password = args[1]
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("DB password updated")
		return "✅ 数据库密码已更新", nil

	default:
		return "", fmt.Errorf("unknown db subkey: %s（可选值: enabled, host, port, name, schema, user, password）", subkey)
	}
}

// dbInit initializes the PostgreSQL database by dropping and recreating all tables.
func (h *SettingsHandler) dbInit() (string, error) {
	if !h.cfg.DB.Enabled {
		return "", fmt.Errorf("数据库连接未启用，请先使用 .db config 配置并启用数据库连接")
	}

	fmt.Print("⚠️  此操作将删除 PostgreSQL 数据库中所有现有数据并重建表结构。是否继续? (y/n, 默认: n): ")
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch strings.ToLower(line) {
		case "y", "yes", "on", "1", "true":
			// Continue
		default:
			return "❌ 已取消初始化", nil
		}
	} else {
		return "❌ 已取消初始化", nil
	}

	fmt.Println("⏳ 正在初始化 PostgreSQL 数据库...")
	pgStore, err := store.NewPGStore(h.cfg.DB)
	if err != nil {
		return "", fmt.Errorf("无法连接 PostgreSQL: %w", err)
	}
	defer pgStore.Close()

	if err := pgStore.DropTables(); err != nil {
		return "", fmt.Errorf("删除表失败: %w", err)
	}

	if err := pgStore.RecreateTables(); err != nil {
		return "", fmt.Errorf("重建表失败: %w", err)
	}

	return "✅ 数据库初始化完成，所有表已重建!", nil
}

// dbMigrate migrates data from local bbolt to PostgreSQL.
func (h *SettingsHandler) dbMigrate() (string, error) {
	if !h.cfg.DB.Enabled {
		return "", fmt.Errorf("数据库连接未启用，请先使用 .db config 配置并启用数据库连接")
	}

	fmt.Println("⚠️  数据迁移说明:")
	fmt.Println("  - schedules、taskplans、session、context 表将全量覆盖 PostgreSQL 中的现有数据")
	fmt.Println("  - history、memory、token_usage 表仅迁移新增数据（增量迁移）")
	fmt.Println("  - 迁移过程中不会删除本地 bbolt 数据")
	fmt.Println("  - 迁移后若数据不一致可能导致记忆混乱，建议先备份本地数据库")
	fmt.Print("\n是否继续执行数据迁移? (y/n, 默认: n): ")
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch strings.ToLower(line) {
		case "y", "yes", "on", "1", "true":
			// Continue
		default:
			return "❌ 已取消数据迁移", nil
		}
	} else {
		return "❌ 已取消数据迁移", nil
	}

	fmt.Println("\n⏳ 正在从本地 bbolt 迁移数据到 PostgreSQL...")
	pgStore, err := store.NewPGStore(h.cfg.DB)
	if err != nil {
		return "", fmt.Errorf("无法连接 PostgreSQL: %w", err)
	}
	defer pgStore.Close()

	if err := pgStore.MigrateFromBolt(h.store); err != nil {
		return "", fmt.Errorf("迁移过程中出现错误: %w", err)
	}

	return "✅ 数据迁移完成!", nil
}

// dbConfigWizard launches an interactive wizard to configure PostgreSQL database
// connection settings. It guides the user through each parameter step by step,
// then offers to test the connection.
func (h *SettingsHandler) dbConfigWizard() (string, error) {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("\n📦 PostgreSQL 数据库配置向导")
	fmt.Println("按 Enter 跳过使用默认值，输入 q 退出向导")
	fmt.Println()

	// Step 1: Enabled
	fmt.Print("是否启用数据库连接? (y/n, 默认: y): ")
	enabled := true
	if scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "q" || line == "quit" {
			return "❌ 已退出数据库配置向导", nil
		}
		switch strings.ToLower(line) {
		case "n", "no", "off", "0", "false":
			enabled = false
		}
	}
	h.cfg.DB.Enabled = enabled

	if !enabled {
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		return "✅ 数据库连接已关闭", nil
	}

	// Step 2: Host
	defaultHost := h.cfg.DB.Host
	if defaultHost == "" {
		defaultHost = "localhost"
	}
	fmt.Printf("数据库主机地址 (默认: %s): ", defaultHost)
	if scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "q" || line == "quit" {
			return "❌ 已退出数据库配置向导", nil
		}
		if line != "" {
			h.cfg.DB.Host = line
		} else {
			h.cfg.DB.Host = defaultHost
		}
	}

	// Step 3: Port
	defaultPort := h.cfg.DB.Port
	if defaultPort == 0 {
		defaultPort = 5432
	}
	fmt.Printf("数据库端口 (默认: %d): ", defaultPort)
	if scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "q" || line == "quit" {
			return "❌ 已退出数据库配置向导", nil
		}
		if line != "" {
			n, err := strconv.Atoi(line)
			if err == nil && n >= 1 && n <= 65535 {
				h.cfg.DB.Port = n
			} else {
				fmt.Printf("⚠️  无效端口号，使用默认值 %d\n", defaultPort)
				h.cfg.DB.Port = defaultPort
			}
		} else {
			h.cfg.DB.Port = defaultPort
		}
	}

	// Step 4: DB Name
	defaultDBName := h.cfg.DB.DBName
	if defaultDBName == "" {
		defaultDBName = "postgres"
	}
	fmt.Printf("数据库名称 (默认: %s): ", defaultDBName)
	if scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "q" || line == "quit" {
			return "❌ 已退出数据库配置向导", nil
		}
		if line != "" {
			h.cfg.DB.DBName = line
		} else {
			h.cfg.DB.DBName = defaultDBName
		}
	}

	// Step 5: Schema - use agent name as default
	defaultSchema := h.cfg.DB.Schema
	if defaultSchema == "" {
		if h.agent != nil && h.agent.Name() != "" {
			defaultSchema = h.agent.Name()
		} else {
			defaultSchema = "public"
		}
	}
	fmt.Printf("数据库 Schema (默认: %s): ", defaultSchema)
	if scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "q" || line == "quit" {
			return "❌ 已退出数据库配置向导", nil
		}
		if line != "" {
			h.cfg.DB.Schema = line
		} else {
			h.cfg.DB.Schema = defaultSchema
		}
	}

	// Step 6: User
	defaultUser := h.cfg.DB.User
	if defaultUser == "" {
		defaultUser = "postgres"
	}
	fmt.Printf("数据库用户 (默认: %s): ", defaultUser)
	if scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "q" || line == "quit" {
			return "❌ 已退出数据库配置向导", nil
		}
		if line != "" {
			h.cfg.DB.User = line
		} else {
			h.cfg.DB.User = defaultUser
		}
	}

	// Step 7: Password
	fmt.Print("数据库密码 (输入后回车，留空保留原值): ")
	if scanner.Scan() {
		line := scanner.Text()
		if line == "q" || line == "quit" {
			return "❌ 已退出数据库配置向导", nil
		}
		if line != "" {
			h.cfg.DB.Password = line
		}
	}

	// Save config first
	if err := h.cfg.Save(); err != nil {
		return "", fmt.Errorf("保存配置失败: %w", err)
	}

	fmt.Println("\n📋 配置摘要:")
	fmt.Printf("  enabled:  %v\n", h.cfg.DB.Enabled)
	fmt.Printf("  host:     %s\n", h.cfg.DB.Host)
	fmt.Printf("  port:     %d\n", h.cfg.DB.Port)
	fmt.Printf("  name:     %s\n", h.cfg.DB.DBName)
	fmt.Printf("  schema:   %s\n", h.cfg.DB.Schema)
	fmt.Printf("  user:     %s\n", h.cfg.DB.User)
	fmt.Printf("  password: ****\n")

	// Step 8: Test connection
	fmt.Print("\n是否测试数据库连接? (y/n, 默认: y): ")
	testConn := true
	if scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "q" || line == "quit" {
			return "❌ 已退出数据库配置向导", nil
		}
		switch strings.ToLower(line) {
		case "n", "no", "off", "0", "false":
			testConn = false
		}
	}

	if testConn {
		fmt.Println("\n🔌 正在测试数据库连接...")
		pgStore, err := store.NewPGStore(h.cfg.DB)
		if err != nil {
			fmt.Printf("❌ 连接失败: %v\n", err)
			fmt.Print("是否忽略错误并保存配置? (y/n, 默认: n): ")
			if scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				switch strings.ToLower(line) {
				case "y", "yes", "on", "1", "true":
					// Keep config as-is
				default:
					return "❌ 配置未保存，请检查连接参数后重试", nil
				}
			}
		} else {
			fmt.Println("✅ 数据库连接成功!")
			pgStore.Close()
		}
	}

	return "✅ 数据库配置完成!", nil
}

// dbBackup exports all PostgreSQL tables to CSV files in backup/<timestamp>/.
func (h *SettingsHandler) dbBackup() (string, error) {
	if !h.cfg.DB.Enabled {
		return "", fmt.Errorf("数据库连接未启用，请先使用 .db config 配置并启用数据库连接")
	}

	pgStore, err := store.NewPGStore(h.cfg.DB)
	if err != nil {
		return "", fmt.Errorf("无法连接 PostgreSQL: %w", err)
	}
	defer pgStore.Close()

	// Create backup directory: backup/<timestamp>/
	timestamp := time.Now().Format("20060102150405")
	backupDir := filepath.Join("backup", timestamp)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("无法创建备份目录 %s: %w", backupDir, err)
	}

	fmt.Printf("⏳ 正在备份数据库到 %s/ ...\n", backupDir)
	if err := pgStore.BackupToCSV(backupDir); err != nil {
		return "", fmt.Errorf("备份失败: %w", err)
	}

	return fmt.Sprintf("✅ 数据库备份完成! 备份文件保存在 %s/", backupDir), nil
}

// dbRestore lists available backups and restores data from a selected one.
func (h *SettingsHandler) dbRestore() (string, error) {
	if !h.cfg.DB.Enabled {
		return "", fmt.Errorf("数据库连接未启用，请先使用 .db config 配置并启用数据库连接")
	}

	// List available backups
	backupBase := "backup"
	entries, err := os.ReadDir(backupBase)
	if err != nil {
		return "", fmt.Errorf("无法读取备份目录 %s/: %w", backupBase, err)
	}

	var backups []string
	for _, entry := range entries {
		if entry.IsDir() {
			backups = append(backups, entry.Name())
		}
	}

	if len(backups) == 0 {
		return "❌ 未找到任何备份", nil
	}

	sort.Sort(sort.Reverse(sort.StringSlice(backups)))

	fmt.Println("可用的备份:")
	for i, b := range backups {
		fmt.Printf("  %d. %s\n", i+1, b)
	}

	fmt.Print("\n请选择要恢复的备份编号 (输入 q 取消): ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return "❌ 已取消恢复", nil
	}
	line := strings.TrimSpace(scanner.Text())
	if line == "q" || line == "quit" {
		return "❌ 已取消恢复", nil
	}

	idx, err := strconv.Atoi(line)
	if err != nil || idx < 1 || idx > len(backups) {
		return "", fmt.Errorf("无效的编号，请输入 1 ~ %d 之间的数字", len(backups))
	}

	selected := backups[idx-1]
	backupDir := filepath.Join(backupBase, selected)

	fmt.Printf("\n⚠️  恢复数据将覆盖 PostgreSQL 数据库中所有现有数据!\n")
	fmt.Printf("   备份来源: %s/\n", backupDir)
	fmt.Print("是否继续恢复? (y/n, 默认: n): ")
	if !scanner.Scan() {
		return "❌ 已取消恢复", nil
	}
	confirm := strings.TrimSpace(scanner.Text())
	switch strings.ToLower(confirm) {
	case "y", "yes", "on", "1", "true":
		// Continue
	default:
		return "❌ 已取消恢复", nil
	}

	pgStore, err := store.NewPGStore(h.cfg.DB)
	if err != nil {
		return "", fmt.Errorf("无法连接 PostgreSQL: %w", err)
	}
	defer pgStore.Close()

	fmt.Println("⏳ 正在恢复数据...")
	if err := pgStore.RestoreFromCSV(backupDir); err != nil {
		return "", fmt.Errorf("恢复失败: %w", err)
	}

	return fmt.Sprintf("✅ 数据恢复完成! 已从 %s/ 恢复数据", backupDir), nil
}
