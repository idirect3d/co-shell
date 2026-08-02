// Author: L.Shuang
// Created: 2026-05-21
// Last Modified: 2026-06-06
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
// IMPLIED, BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/store"
)

// readLineFromIO reads a line from UserIO.
func readLineFromIO(io agent.UserIO) string {
	line, err := io.ReadLine()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(line)
}

// HandleDB handles the .db built-in command.
// Usage:
//
//	.db              - Show current database configuration
//	.db config       - Launch the database configuration wizard
//	.db init         - Initialize PostgreSQL database (drop and recreate all tables)
//	.db sync         - Sync memory and history from local bbolt to PostgreSQL
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
	case "sync":
		// .db sync -> sync data from bbolt to PostgreSQL
		return h.dbSync()
	case "backup":
		// .db backup -> backup all tables to CSV
		return h.dbBackup()
	case "restore":
		// .db restore -> restore from backup
		return h.dbRestore()
	case "status":
		// .db status -> re-test connection and show status
		return h.dbCheckStatus()
	default:
		// .db <subkey> <value> -> delegate to handleDBSubCommand
		return h.handleDBSubCommand(args)
	}
}

// showDBConfig displays the current DB configuration, connection status, and usage instructions.
func (h *SettingsHandler) showDBConfig() (string, error) {
	enabledStatus := i18n.T(i18n.KeyOff)
	if h.cfg.DB.Enabled {
		enabledStatus = i18n.T(i18n.KeyOn)
	}

	// Determine connection status
	connStatus := i18n.T(i18n.KeyDBStatusNone)
	if h.cfg.DB.Enabled && h.cfg.DB.Host != "" && h.cfg.DB.Port > 0 && h.cfg.DB.DBName != "" {
		pgStore, err := store.NewPGStore(h.cfg.DB)
		if err != nil {
			connStatus = i18n.T(i18n.KeyDBStatusFailed)
		} else {
			pgStore.Close()
			connStatus = i18n.T(i18n.KeyDBStatusConnected)
		}
	}

	var sb strings.Builder
	sb.WriteString(i18n.T(i18n.KeyDBConfigLabel) + ":\n")
	sb.WriteString(fmt.Sprintf("  %-20s %-20s %s\n", "status:", connStatus, i18n.T(i18n.KeyDBStatusLabel)))
	sb.WriteString(fmt.Sprintf("  %-20s %-20s %s\n", "enabled:", enabledStatus, i18n.T(i18n.KeyDBEnabledLabel)))
	sb.WriteString(fmt.Sprintf("  %-20s %-20s %s\n", "host:", h.cfg.DB.Host, i18n.T(i18n.KeyDBHostLabel)))
	sb.WriteString(fmt.Sprintf("  %-20s %-20d %s\n", "port:", h.cfg.DB.Port, i18n.T(i18n.KeyDBPortLabel)))
	sb.WriteString(fmt.Sprintf("  %-20s %-20s %s\n", "name:", h.cfg.DB.DBName, i18n.T(i18n.KeyDBNameLabel)))
	sb.WriteString(fmt.Sprintf("  %-20s %-20s %s\n", "schema:", h.cfg.DB.Schema, i18n.T(i18n.KeyDBSchemaLabel)))
	sb.WriteString(fmt.Sprintf("  %-20s %-20s %s\n", "user:", h.cfg.DB.User, i18n.T(i18n.KeyDBUserLabel)))
	sb.WriteString(fmt.Sprintf("  %-20s %-20s %s\n", "password:", "****", i18n.T(i18n.KeyDBPasswordLabel)))

	// Display timeout
	timeoutStr := fmt.Sprintf("%ds", h.cfg.DB.Timeout)
	if h.cfg.DB.Timeout <= 0 {
		timeoutStr = i18n.T(i18n.KeyUnlimited)
	}
	sb.WriteString(fmt.Sprintf("  %-20s %-20s %s\n", "timeout:", timeoutStr, i18n.T(i18n.KeyDBTimeoutLabel)))

	autoSyncStatus := i18n.T(i18n.KeyOn)
	if !h.cfg.DB.AutoSync {
		autoSyncStatus = i18n.T(i18n.KeyOff)
	}
	sb.WriteString(fmt.Sprintf("  %-20s %-20s %s\n", "auto-sync:", autoSyncStatus, i18n.T(i18n.KeyDBAutoSyncDesc)))

	sb.WriteString("\n.set db <key> <value> - " + i18n.T(i18n.KeyDBSubCmdDesc) + "\n")
	sb.WriteString(".db config - " + i18n.T(i18n.KeyDBConfigLabel) + "\n")
	sb.WriteString(".db status - " + i18n.T(i18n.KeyDBStatusCmd) + "\n")
	sb.WriteString(".db init - " + i18n.T(i18n.KeyDBInitDesc) + "\n")
	sb.WriteString(".db sync - " + i18n.T(i18n.KeyDBMigrateDescMemory) + "\n")
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
			return fmt.Sprintf(i18n.T(i18n.KeyDBEnabledVal), status), nil
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
			return fmt.Sprintf(i18n.T(i18n.KeyDBEnabledSet), status), nil
		}

		// When enabling DB, immediately test the connection with current parameters
		log.Info("DB enabled set to %s", status)
		h.io().Println(i18n.T(i18n.KeyDBTestingConn))
		pgStore, err := store.NewPGStore(h.cfg.DB)
		if err != nil {
			h.io().Printf(i18n.T(i18n.KeyDBConnFailed), err)
			h.io().Print(i18n.T(i18n.KeyDBInitWizardPrompt))
			line := readLineFromIO(h.io())
			switch strings.ToLower(line) {
			case "n", "no", "off", "0", "false":
				return i18n.T(i18n.KeyDBEnabledNotTested), nil
			default:
				return h.dbConfigWizard()
			}
		}
		pgStore.Close()
		h.io().Println(i18n.T(i18n.KeyDBConnOK))
		return fmt.Sprintf(i18n.T(i18n.KeyDBEnabledSet), status), nil

	case "host":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeyDBHostVal), h.cfg.DB.Host), nil
		}
		h.cfg.DB.Host = args[1]
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("DB host set to %s", args[1])
		return fmt.Sprintf(i18n.T(i18n.KeyDBHostSet), args[1]), nil

	case "port":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeyDBPortVal), h.cfg.DB.Port), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf(i18n.T(i18n.KeyDBInvalidPort), args[1])
		}
		if n < 1 || n > 65535 {
			return "", errors.New(i18n.T(i18n.KeyDBPortRange))
		}
		h.cfg.DB.Port = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("DB port set to %d", n)
		return fmt.Sprintf(i18n.T(i18n.KeyDBPortSet), n), nil

	case "name":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeyDBNameVal), h.cfg.DB.DBName), nil
		}
		h.cfg.DB.DBName = args[1]
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("DB name set to %s", args[1])
		return fmt.Sprintf(i18n.T(i18n.KeyDBNameSet), args[1]), nil

	case "schema":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeyDBSchemaVal), h.cfg.DB.Schema), nil
		}
		h.cfg.DB.Schema = args[1]
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("DB schema set to %s", args[1])
		return fmt.Sprintf(i18n.T(i18n.KeyDBSchemaSet), args[1]), nil

	case "user":
		if len(args) < 2 {
			return fmt.Sprintf(i18n.T(i18n.KeyDBUserVal), h.cfg.DB.User), nil
		}
		h.cfg.DB.User = args[1]
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("DB user set to %s", args[1])
		return fmt.Sprintf(i18n.T(i18n.KeyDBUserSet), args[1]), nil

	case "password":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: .set db password <password>")
		}
		h.cfg.DB.Password = args[1]
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("DB password updated")
		return i18n.T(i18n.KeyDBPasswordSet), nil

	case "timeout":
		if len(args) < 2 {
			timeoutStr := fmt.Sprintf("%ds", h.cfg.DB.Timeout)
			if h.cfg.DB.Timeout <= 0 {
				timeoutStr = i18n.T(i18n.KeyUnlimitedLower)
			}
			return fmt.Sprintf(i18n.T(i18n.KeyDBTimeoutVal), timeoutStr), nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 0 {
			return "", fmt.Errorf("usage: .set db timeout <seconds> (>= 0, 0 = no timeout)")
		}
		h.cfg.DB.Timeout = n
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		log.Info("DB timeout set to %d", n)
		return fmt.Sprintf(i18n.T(i18n.KeyDBTimeoutSet), n), nil

	case "auto-sync":
		if len(args) < 2 {
			status := i18n.T(i18n.KeyOff)
			if h.cfg.DB.AutoSync {
				status = i18n.T(i18n.KeyOn)
			}
			return fmt.Sprintf(i18n.T(i18n.KeyDBAutoSyncVal), status), nil
		}
		switch args[1] {
		case "on", "1", "true", "yes":
			h.cfg.DB.AutoSync = true
		case "off", "0", "false", "no":
			h.cfg.DB.AutoSync = false
		default:
			return "", fmt.Errorf("usage: .set db auto-sync on|off")
		}
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		status := i18n.T(i18n.KeyOn)
		if !h.cfg.DB.AutoSync {
			status = i18n.T(i18n.KeyOff)
		}
		log.Info("DB auto-sync set to %s", status)
		return fmt.Sprintf(i18n.T(i18n.KeyDBAutoSyncSet), status), nil

	default:
		return "", fmt.Errorf(i18n.T(i18n.KeyDBUnknownSubkey), subkey)
	}
}

// dbInit initializes the PostgreSQL database by dropping and recreating all tables.
func (h *SettingsHandler) dbInit() (string, error) {
	if !h.cfg.DB.Enabled {
		return "", errors.New(i18n.T(i18n.KeyDBNotEnabled))
	}

	h.io().Print(i18n.T(i18n.KeyDBInitConfirm))
	line := readLineFromIO(h.io())
	switch strings.ToLower(line) {
	case "y", "yes", "on", "1", "true":
		// Continue
	default:
		return i18n.T(i18n.KeyDBCancelledInit), nil
	}

	h.io().Println(i18n.T(i18n.KeyDBInitProgress))
	pgStore, err := store.NewPGStore(h.cfg.DB)
	if err != nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyDBCannotConnect), err)
	}
	defer pgStore.Close()

	if err := pgStore.DropTables(); err != nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyDBDropFailed), err)
	}

	if err := pgStore.RecreateTables(); err != nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyDBRecreateFailed), err)
	}

	return i18n.T(i18n.KeyDBInitDone), nil
}

// dbSync syncs memory and history data from local bbolt to PostgreSQL.
func (h *SettingsHandler) dbSync() (string, error) {
	if !h.cfg.DB.Enabled {
		return "", errors.New(i18n.T(i18n.KeyDBNotEnabled))
	}

	h.io().Println(i18n.T(i18n.KeyDBSyncExplain))
	h.io().Println(i18n.T(i18n.KeyDBSyncOnlyMem))
	h.io().Println(i18n.T(i18n.KeyDBSyncIncremental))
	h.io().Println(i18n.T(i18n.KeyDBSyncNoDelete))
	h.io().Println(i18n.T(i18n.KeyDBSyncOnlyLocal))
	h.io().Print(i18n.T(i18n.KeyDBSyncConfirm))
	line := readLineFromIO(h.io())
	switch strings.ToLower(line) {
	case "y", "yes", "on", "1", "true":
		// Continue
	default:
		return i18n.T(i18n.KeyDBCancelledSync), nil
	}

	h.io().Println(i18n.T(i18n.KeyDBSyncProgress))
	pgStore, err := store.NewPGStore(h.cfg.DB)
	if err != nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyDBCannotConnect), err)
	}
	defer pgStore.Close()

	if err := pgStore.MigrateFromBolt(h.store.Bolt); err != nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyDBSyncError), err)
	}

	return i18n.T(i18n.KeyDBSyncDone), nil
}

// dbConfigWizard launches an interactive wizard to configure PostgreSQL database
// connection settings. It guides the user through each parameter step by step,
// then offers to test the connection.
func (h *SettingsHandler) dbConfigWizard() (string, error) {
	io := h.io()

	io.Println(i18n.T(i18n.KeyDBWizardTitle))
	io.Println(i18n.T(i18n.KeyDBWizardExitHint))
	io.Println()

	// Step 1: Enabled
	io.Print(i18n.T(i18n.KeyDBEnablePrompt))
	enabled := true
	line := readLineFromIO(io)
	if line == "q" || line == "quit" {
		return i18n.T(i18n.KeyDBWizardExited), nil
	}
	switch strings.ToLower(line) {
	case "n", "no", "off", "0", "false":
		enabled = false
	}
	h.cfg.DB.Enabled = enabled

	if !enabled {
		if err := h.cfg.Save(); err != nil {
			return "", err
		}
		return i18n.T(i18n.KeyDBDisabled), nil
	}

	// Step 2: Host
	defaultHost := h.cfg.DB.Host
	if defaultHost == "" {
		defaultHost = "localhost"
	}
	io.Printf(i18n.T(i18n.KeyDBHostPrompt), defaultHost)
	line = readLineFromIO(io)
	if line == "q" || line == "quit" {
		return i18n.T(i18n.KeyDBWizardExited), nil
	}
	if line != "" {
		h.cfg.DB.Host = line
	} else {
		h.cfg.DB.Host = defaultHost
	}

	// Step 3: Port
	defaultPort := h.cfg.DB.Port
	if defaultPort == 0 {
		defaultPort = 5432
	}
	io.Printf(i18n.T(i18n.KeyDBPortPrompt), defaultPort)
	line = readLineFromIO(io)
	if line == "q" || line == "quit" {
		return i18n.T(i18n.KeyDBWizardExited), nil
	}
	if line != "" {
		n, err := strconv.Atoi(line)
		if err == nil && n >= 1 && n <= 65535 {
			h.cfg.DB.Port = n
		} else {
			io.Printf(i18n.T(i18n.KeyDBInvalidPortDef), defaultPort)
			h.cfg.DB.Port = defaultPort
		}
	} else {
		h.cfg.DB.Port = defaultPort
	}

	// Step 4: DB Name
	defaultDBName := h.cfg.DB.DBName
	if defaultDBName == "" {
		defaultDBName = "postgres"
	}
	io.Printf(i18n.T(i18n.KeyDBNamePrompt), defaultDBName)
	line = readLineFromIO(io)
	if line == "q" || line == "quit" {
		return i18n.T(i18n.KeyDBWizardExited), nil
	}
	if line != "" {
		h.cfg.DB.DBName = line
	} else {
		h.cfg.DB.DBName = defaultDBName
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
	io.Printf(i18n.T(i18n.KeyDBSchemaPrompt), defaultSchema)
	line = readLineFromIO(io)
	if line == "q" || line == "quit" {
		return i18n.T(i18n.KeyDBWizardExited), nil
	}
	if line != "" {
		h.cfg.DB.Schema = line
	} else {
		h.cfg.DB.Schema = defaultSchema
	}

	// Step 6: User
	defaultUser := h.cfg.DB.User
	if defaultUser == "" {
		defaultUser = "postgres"
	}
	io.Printf(i18n.T(i18n.KeyDBUserPrompt), defaultUser)
	line = readLineFromIO(io)
	if line == "q" || line == "quit" {
		return i18n.T(i18n.KeyDBWizardExited), nil
	}
	if line != "" {
		h.cfg.DB.User = line
	} else {
		h.cfg.DB.User = defaultUser
	}

	// Step 7: Password
	io.Print(i18n.T(i18n.KeyDBPasswordPrompt))
	lineRaw, _ := io.ReadLine()
	line = strings.TrimSpace(lineRaw)
	if line == "q" || line == "quit" {
		return i18n.T(i18n.KeyDBWizardExited), nil
	}
	if line != "" {
		h.cfg.DB.Password = line
	}

	// Save config first
	if err := h.cfg.Save(); err != nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyDBConfigFailed), err)
	}

	io.Println(i18n.T(i18n.KeyDBSummary))
	io.Printf("  enabled:  %v\n", h.cfg.DB.Enabled)
	io.Printf("  host:     %s\n", h.cfg.DB.Host)
	io.Printf("  port:     %d\n", h.cfg.DB.Port)
	io.Printf("  name:     %s\n", h.cfg.DB.DBName)
	io.Printf("  schema:   %s\n", h.cfg.DB.Schema)
	io.Printf("  user:     %s\n", h.cfg.DB.User)
	io.Printf("  password: ****\n")

	// Step 8: Test connection
	io.Print(i18n.T(i18n.KeyDBTestConnPrompt))
	testConn := true
	line = readLineFromIO(io)
	if line == "q" || line == "quit" {
		return i18n.T(i18n.KeyDBWizardExited), nil
	}
	switch strings.ToLower(line) {
	case "n", "no", "off", "0", "false":
		testConn = false
	}

	if testConn {
		io.Println(i18n.T(i18n.KeyDBTestingConn))
		pgStore, err := store.NewPGStore(h.cfg.DB)
		if err != nil {
			io.Printf(i18n.T(i18n.KeyDBConnFailMsg), err)
			io.Print(i18n.T(i18n.KeyDBIgnoreSavePrompt))
			confirm := readLineFromIO(io)
			switch strings.ToLower(confirm) {
			case "y", "yes", "on", "1", "true":
				// Keep config as-is
			default:
				return i18n.T(i18n.KeyDBNotSaved), nil
			}
		} else {
			io.Println(i18n.T(i18n.KeyDBConnOK))
			pgStore.Close()
		}
	}

	return i18n.T(i18n.KeyDBConfigDone), nil
}

// dbCheckStatus re-tests the database connection and displays the current status.
func (h *SettingsHandler) dbCheckStatus() (string, error) {
	connStatus := i18n.T(i18n.KeyDBStatusNone)
	if h.cfg.DB.Enabled && h.cfg.DB.Host != "" && h.cfg.DB.Port > 0 && h.cfg.DB.DBName != "" {
		h.io().Println(i18n.T(i18n.KeyDBTestingConn))
		pgStore, err := store.NewPGStore(h.cfg.DB)
		if err != nil {
			h.io().Printf("❌ %v\n", err)
			connStatus = i18n.T(i18n.KeyDBStatusFailed)
		} else {
			pgStore.Close()
			connStatus = i18n.T(i18n.KeyDBStatusConnected)
			h.io().Println("✅ " + i18n.T(i18n.KeyDBStatusConnected))
		}
	}
	return fmt.Sprintf(i18n.T(i18n.KeyDBConnStatusTitle), connStatus), nil
}

// dbBackup exports all PostgreSQL tables to CSV files in backup/<timestamp>/.
func (h *SettingsHandler) dbBackup() (string, error) {
	if !h.cfg.DB.Enabled {
		return "", errors.New(i18n.T(i18n.KeyDBNotEnabled))
	}

	pgStore, err := store.NewPGStore(h.cfg.DB)
	if err != nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyDBCannotConnect), err)
	}
	defer pgStore.Close()

	// Create backup directory: backup/<timestamp>/
	timestamp := time.Now().Format("20060102150405")
	backupDir := filepath.Join("backup", timestamp)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyDBBackupDirFailed), backupDir, err)
	}

	h.io().Printf(i18n.T(i18n.KeyDBBackupProgress), backupDir)
	if err := pgStore.BackupToCSV(backupDir); err != nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyDBBackupFailedMsg), err)
	}

	return fmt.Sprintf(i18n.T(i18n.KeyDBBackupDone), backupDir), nil
}

// dbRestore lists available backups and restores data from a selected one.
func (h *SettingsHandler) dbRestore() (string, error) {
	if !h.cfg.DB.Enabled {
		return "", errors.New(i18n.T(i18n.KeyDBNotEnabled))
	}

	// List available backups
	backupBase := "backup"
	entries, err := os.ReadDir(backupBase)
	if err != nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyDBBackupDirFailed), backupBase, err)
	}

	var backups []string
	for _, entry := range entries {
		if entry.IsDir() {
			backups = append(backups, entry.Name())
		}
	}

	if len(backups) == 0 {
		return i18n.T(i18n.KeyDBNoBackupFound), nil
	}

	sort.Sort(sort.Reverse(sort.StringSlice(backups)))

	h.io().Println(i18n.T(i18n.KeyDBRestoreList))
	for i, b := range backups {
		h.io().Printf("  %d. %s\n", i+1, b)
	}

	h.io().Print(i18n.T(i18n.KeyDBRestorePrompt))
	line := readLineFromIO(h.io())
	if line == "q" || line == "quit" {
		return i18n.T(i18n.KeyDBCancelledRestore), nil
	}

	idx, err := strconv.Atoi(line)
	if err != nil || idx < 1 || idx > len(backups) {
		return "", fmt.Errorf(i18n.T(i18n.KeyDBInvalidIndex), len(backups))
	}

	selected := backups[idx-1]
	backupDir := filepath.Join(backupBase, selected)

	h.io().Printf(i18n.T(i18n.KeyDBRestoreWarningB))
	h.io().Printf(i18n.T(i18n.KeyDBRestoreSource), backupDir)
	h.io().Print(i18n.T(i18n.KeyDBRestoreConfirmB))
	confirm := readLineFromIO(h.io())
	switch strings.ToLower(confirm) {
	case "y", "yes", "on", "1", "true":
		// Continue
	default:
		return i18n.T(i18n.KeyDBCancelledRestore), nil
	}

	pgStore, err := store.NewPGStore(h.cfg.DB)
	if err != nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyDBCannotConnect), err)
	}
	defer pgStore.Close()

	h.io().Println(i18n.T(i18n.KeyDBRestoreProgress))
	if err := pgStore.RestoreFromCSV(backupDir); err != nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyDBRestoreFailedMsg), err)
	}

	return fmt.Sprintf(i18n.T(i18n.KeyDBRestoreDoneMsg), backupDir), nil
}
