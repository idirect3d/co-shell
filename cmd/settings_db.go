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
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/workspace"
)

// handleDBSubCommand handles the .set db <subkey> <value> sub-command.
// When called with no arguments and all DB config fields are at their default
// (empty) values, it launches an interactive configuration wizard.
func (h *SettingsHandler) handleDBSubCommand(args []string) (string, error) {
	if len(args) == 0 {
		// Check if all DB config fields are at defaults (empty)
		if h.cfg.DB.Host == "" && h.cfg.DB.Port == 0 && h.cfg.DB.DBName == "" &&
			h.cfg.DB.User == "" && h.cfg.DB.Password == "" {
			return h.dbConfigWizard()
		}
		// Show all DB settings
		enabledStatus := i18n.T(i18n.KeyOff)
		if h.cfg.DB.Enabled {
			enabledStatus = i18n.T(i18n.KeyOn)
		}
		var sb strings.Builder
		sb.WriteString("数据库配置:\n")
		sb.WriteString(fmt.Sprintf("  enabled:  %s\n", enabledStatus))
		sb.WriteString(fmt.Sprintf("  host:     %s\n", h.cfg.DB.Host))
		sb.WriteString(fmt.Sprintf("  port:     %d\n", h.cfg.DB.Port))
		sb.WriteString(fmt.Sprintf("  name:     %s\n", h.cfg.DB.DBName))
		sb.WriteString(fmt.Sprintf("  schema:   %s\n", h.cfg.DB.Schema))
		sb.WriteString(fmt.Sprintf("  user:     %s\n", h.cfg.DB.User))
		sb.WriteString(fmt.Sprintf("  password: ****\n"))
		return sb.String(), nil
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

// dbConfigWizard launches an interactive wizard to configure PostgreSQL database
// connection settings. It guides the user through each parameter step by step,
// then offers to test the connection and optionally migrate data from bbolt.
func (h *SettingsHandler) dbConfigWizard() (string, error) {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("\n📦 PostgreSQL 数据库配置向导")
	fmt.Println("按 Enter 跳过使用默认值，输入 q 退出向导")
	fmt.Println()

	// Step 1: Enabled
	fmt.Print("是否启用数据库连接? (y/n, 默认: n): ")
	enabled := false
	if scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "q" || line == "quit" {
			return "❌ 已退出数据库配置向导", nil
		}
		switch strings.ToLower(line) {
		case "y", "yes", "on", "1", "true":
			enabled = true
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
		defaultDBName = "coshell"
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

	// Step 5: Schema - use current directory name as default
	defaultSchema := h.cfg.DB.Schema
	if defaultSchema == "" {
		cwd, err := os.Getwd()
		if err == nil {
			defaultSchema = filepath.Base(cwd)
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
	fmt.Print("数据库密码 (输入后回车): ")
	if scanner.Scan() {
		line := scanner.Text()
		if line == "q" || line == "quit" {
			return "❌ 已退出数据库配置向导", nil
		}
		h.cfg.DB.Password = line
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

			// Step 9: Auto-migrate from bbolt
			fmt.Print("\n是否从本地 bbolt 数据库迁移数据到 PostgreSQL? (y/n, 默认: n): ")
			if scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				switch strings.ToLower(line) {
				case "y", "yes", "on", "1", "true":
					fmt.Println("⏳ 正在迁移数据...")
					// Open bbolt store from current workspace
					ws, err := workspace.New("")
					if err != nil {
						fmt.Printf("⚠️  无法创建工作区: %v\n", err)
					} else {
						boltStore, err := store.NewStore(ws)
						if err != nil {
							fmt.Printf("⚠️  无法打开本地 bbolt 数据库: %v\n", err)
						} else {
							// Re-open PG store for migration
							pgStore2, err := store.NewPGStore(h.cfg.DB)
							if err != nil {
								fmt.Printf("⚠️  无法重新连接 PostgreSQL: %v\n", err)
							} else {
								if err := pgStore2.MigrateFromBolt(boltStore); err != nil {
									fmt.Printf("⚠️  迁移过程中出现错误: %v\n", err)
								} else {
									fmt.Println("✅ 数据迁移完成!")
								}
								pgStore2.Close()
							}
							boltStore.Close()
						}
					}
				}
			}
		}
	}

	return "✅ 数据库配置完成!", nil
}
