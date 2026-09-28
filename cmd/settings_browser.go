// Author: L.Shuang
// Created: 2026-09-26
// Last Modified: 2026-09-26
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
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
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/browser"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// browserSettingNames lists the FEATURE-556 Phase 2 `browser-*` keys handled by
// handleBrowserSetting. The four pre-existing browser keys (browser-enabled,
// browser-port, browser-headless, browser-max-html-size) keep their own cases in
// settings_agent.go and are deliberately not listed here.
var browserSettingNames = map[string]bool{
	"browser-chrome-path":        true,
	"browser-extra-args":         true,
	"browser-proxy":              true,
	"browser-user-agent":         true,
	"browser-language":           true,
	"browser-timezone":           true,
	"browser-extra-http-headers": true,
	"browser-window-size":        true,
	"browser-window-mode":        true,
	"browser-download-dir":       true,
	"browser-ignore-cert-errors": true,
	"browser-extensions":         true,
	"browser-profile-dir":        true,
	"browser-persist":            true,
	"browser-viewport":           true,
	"browser-dialog-policy":      true,
	"browser-apply-on-connect":   true,
}

// isBrowserSettingName reports whether subcommand is one of the FEATURE-556
// browser settings handled by handleBrowserSetting.
func isBrowserSettingName(subcommand string) bool {
	return browserSettingNames[subcommand]
}

// handleBrowserSetting reads or updates one FEATURE-556 `browser-*` setting.
// With no value it reports the current value; otherwise the value is stored and
// the configuration is saved to disk so it survives a co-shell restart.
func (h *SettingsHandler) handleBrowserSetting(subcommand string, args []string) (string, error) {
	switch subcommand {
	case "browser-chrome-path":
		return h.setBrowserString(subcommand, &h.cfg.LLM.BrowserChromePath, args)
	case "browser-proxy":
		return h.setBrowserString(subcommand, &h.cfg.LLM.BrowserProxy, args)
	case "browser-user-agent":
		return h.setBrowserString(subcommand, &h.cfg.LLM.BrowserUserAgent, args)
	case "browser-language":
		return h.setBrowserString(subcommand, &h.cfg.LLM.BrowserLanguage, args)
	case "browser-timezone":
		return h.setBrowserString(subcommand, &h.cfg.LLM.BrowserTimezone, args)
	case "browser-download-dir":
		return h.setBrowserString(subcommand, &h.cfg.LLM.BrowserDownloadDir, args)
	case "browser-profile-dir":
		return h.setBrowserString(subcommand, &h.cfg.LLM.BrowserProfileDir, args)
	case "browser-extra-args":
		return h.setBrowserList(subcommand, &h.cfg.LLM.BrowserExtraArgs, args, false)
	case "browser-extra-http-headers":
		return h.setBrowserList(subcommand, &h.cfg.LLM.BrowserExtraHTTPHeaders, args, true)
	case "browser-window-size":
		return h.setBrowserWindowSize(subcommand, args)
	case "browser-window-mode":
		return h.setBrowserWindowMode(subcommand, args)
	case "browser-viewport":
		return h.setBrowserViewport(subcommand, args)
	case "browser-dialog-policy":
		return h.setBrowserDialogPolicy(subcommand, args)
	case "browser-ignore-cert-errors":
		return h.setBrowserBool(subcommand, &h.cfg.LLM.BrowserIgnoreCertErrors, args)
	case "browser-extensions":
		return h.setBrowserBool(subcommand, &h.cfg.LLM.BrowserExtensions, args)
	case "browser-persist":
		return h.setBrowserBool(subcommand, &h.cfg.LLM.BrowserPersist, args)
	case "browser-apply-on-connect":
		return h.setBrowserBool(subcommand, &h.cfg.LLM.BrowserApplyOnConnect, args)
	default:
		return "", fmt.Errorf("unknown browser setting: %s", subcommand)
	}
}

// setBrowserString reports or updates a plain string browser setting. Values
// may contain spaces: everything after the key is joined back together.
func (h *SettingsHandler) setBrowserString(name string, target *string, args []string) (string, error) {
	if len(args) < 2 {
		return browserSettingLine(name, *target), nil
	}
	value := strings.Join(args[1:], " ")
	if value == "none" {
		value = ""
	}
	*target = value
	return h.saveBrowserSetting(name, *target)
}

// setBrowserList reports or updates a list valued browser setting. When
// commaSeparated is true the value is split on commas (used for headers, whose
// values may contain spaces); otherwise every remaining argument is one entry.
// A single "none" entry clears the list.
func (h *SettingsHandler) setBrowserList(name string, target *[]string, args []string, commaSeparated bool) (string, error) {
	if len(args) < 2 {
		return browserSettingLine(name, strings.Join(*target, ", ")), nil
	}

	var items []string
	if commaSeparated {
		items = strings.Split(strings.Join(args[1:], " "), ",")
	} else {
		items = args[1:]
	}

	cleaned := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		cleaned = append(cleaned, item)
	}
	if len(cleaned) == 1 && cleaned[0] == "none" {
		cleaned = nil
	}

	*target = cleaned
	return h.saveBrowserSetting(name, strings.Join(cleaned, ", "))
}

// setBrowserBool reports or updates an on/off browser setting.
func (h *SettingsHandler) setBrowserBool(name string, target *bool, args []string) (string, error) {
	if len(args) < 2 {
		return browserSettingLine(name, browserOnOff(*target)), nil
	}
	switch strings.ToLower(args[1]) {
	case "on", "1", "true", "yes":
		*target = true
	case "off", "0", "false", "no":
		*target = false
	default:
		return "", fmt.Errorf("usage: .set %s on|off", name)
	}
	return h.saveBrowserSetting(name, browserOnOff(*target))
}

// setBrowserWindowSize reports or updates the Chrome launch window size.
func (h *SettingsHandler) setBrowserWindowSize(name string, args []string) (string, error) {
	if len(args) < 2 {
		return browserSettingLine(name, h.cfg.LLM.BrowserWindowSize), nil
	}
	value := strings.Join(args[1:], "")
	if value == "none" {
		value = ""
	} else if !validSizePair(value, ",") {
		return "", errors.New(".set browser-window-size WIDTH,HEIGHT (e.g. 1280,800)")
	}
	h.cfg.LLM.BrowserWindowSize = value
	return h.saveBrowserSetting(name, value)
}

// setBrowserWindowMode reports or updates the initial window state applied to a
// newly launched non-headless browser (FEATURE-569). An unknown value is
// rejected so the stored configuration always stays usable.
func (h *SettingsHandler) setBrowserWindowMode(name string, args []string) (string, error) {
	if len(args) < 2 {
		return browserSettingLine(name, h.cfg.LLM.BrowserWindowMode), nil
	}
	value := strings.TrimSpace(strings.Join(args[1:], ""))
	switch value {
	case browser.WindowModeMinimized, browser.WindowModeMaximized, browser.WindowModeNormal:
	default:
		return "", errors.New(".set browser-window-mode minimized|maximized|normal")
	}
	h.cfg.LLM.BrowserWindowMode = value
	return h.saveBrowserSetting(name, value)
}

// setBrowserViewport reports or updates the emulated viewport.
func (h *SettingsHandler) setBrowserViewport(name string, args []string) (string, error) {
	if len(args) < 2 {
		return browserSettingLine(name, h.cfg.LLM.BrowserViewport), nil
	}
	value := strings.Join(args[1:], "")
	if value == "none" {
		value = ""
	} else if !validSizePair(value, "x") {
		return "", errors.New(".set browser-viewport WIDTHxHEIGHT (e.g. 1440x900)")
	}
	h.cfg.LLM.BrowserViewport = value
	return h.saveBrowserSetting(name, value)
}

// setBrowserDialogPolicy reports or updates the JavaScript dialog policy.
func (h *SettingsHandler) setBrowserDialogPolicy(name string, args []string) (string, error) {
	if len(args) < 2 {
		return browserSettingLine(name, h.cfg.LLM.BrowserDialogPolicy), nil
	}
	value := strings.ToLower(strings.Join(args[1:], ""))
	switch value {
	case "auto", "accept", "dismiss":
	default:
		return "", errors.New(".set browser-dialog-policy auto|accept|dismiss")
	}
	h.cfg.LLM.BrowserDialogPolicy = value
	return h.saveBrowserSetting(name, value)
}

// saveBrowserSetting persists the configuration and reports the new value.
func (h *SettingsHandler) saveBrowserSetting(name, value string) (string, error) {
	if err := h.cfg.Save(); err != nil {
		return "", err
	}
	log.Info("Browser setting %s set to %q", name, value)
	return i18n.TF(i18n.KeySettingsUpdated, name, value), nil
}

// browserSettingLine renders the "current value" answer for a browser setting.
func browserSettingLine(name, value string) string {
	if strings.TrimSpace(value) == "" {
		value = "(unset)"
	}
	return name + " = " + value
}

// browserOnOff renders a boolean as the localised on/off word.
func browserOnOff(value bool) string {
	if value {
		return i18n.T(i18n.KeyOn)
	}
	return i18n.T(i18n.KeyOff)
}

// validSizePair reports whether value is two positive integers separated by sep
// (e.g. "1280,800" with sep "," or "1440x900" with sep "x").
func validSizePair(value, sep string) bool {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(value)), sep)
	if len(parts) != 2 {
		return false
	}
	for _, part := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || n <= 0 {
			return false
		}
	}
	return true
}

// browserDisplayValue renders a possibly empty browser setting for the status table.
func browserDisplayValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

// browserListDisplay renders a list valued browser setting for the status table.
func browserListDisplay(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ", ")
}

// browserListValue renders a list valued browser setting for the Web settings
// panel, where an empty list is shown as an empty input.
func browserListValue(values []string) string {
	return strings.Join(values, " ")
}
