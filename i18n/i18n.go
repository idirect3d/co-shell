// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-04-25
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
// Package i18n provides internationalization support for co-shell.
// It supports Chinese (zh) and English (en) with easy extensibility.
//
// Language selection priority:
//  1. --lang CLI flag (highest priority)
//  2. LANG / LC_ALL environment variable
//  3. Default to Chinese (zh)
package i18n

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

// Lang defines a language code.
type Lang string

const (
	LangZH Lang = "zh"
	LangEN Lang = "en"
)

// currentLang holds the currently active language.
var (
	mu          sync.RWMutex
	currentLang Lang = LangZH
)

// DetectLang detects the user's preferred language from environment.
func DetectLang() Lang {
	env := os.Getenv("LANG")
	if env == "" {
		env = os.Getenv("LC_ALL")
	}
	if env == "" {
		return LangZH
	}

	env = strings.ToLower(env)
	switch {
	case strings.HasPrefix(env, "zh"):
		return LangZH
	case strings.HasPrefix(env, "en"):
		return LangEN
	default:
		return LangZH
	}
}

// SetLang sets the current language from a string code.
// Returns true if the language is supported.
func SetLang(code string) bool {
	mu.Lock()
	defer mu.Unlock()

	switch strings.ToLower(code) {
	case "zh", "zh-cn", "zh_cn", "chinese":
		currentLang = LangZH
		return true
	case "en", "en-us", "en_us", "english":
		currentLang = LangEN
		return true
	default:
		return false
	}
}

// GetLang returns the current language code.
func GetLang() Lang {
	mu.RLock()
	defer mu.RUnlock()
	return currentLang
}

// Init initializes the i18n system.
// If langFlag is provided and valid, it takes highest priority.
// Otherwise, detects from environment.
func Init(langFlag string) {
	if langFlag != "" && SetLang(langFlag) {
		return
	}
	_ = SetLang(string(DetectLang()))
}

// T returns the translated string for the given key.
// If the key is not found, returns the key itself as fallback.
func T(key string) string {
	mu.RLock()
	lang := currentLang
	mu.RUnlock()

	if msg := lookup(lang, key); msg != "" {
		return msg
	}

	// Fallback to Chinese
	if msg := lookup(LangZH, key); msg != "" {
		return msg
	}

	return key
}

// TF returns the translated string with fmt.Sprintf-style formatting.
func TF(key string, args ...interface{}) string {
	msg := T(key)
	if len(args) > 0 {
		return fmt.Sprintf(msg, args...)
	}
	return msg
}

// lookup finds a translation for a given language and key.
func lookup(lang Lang, key string) string {
	switch lang {
	case LangZH:
		if msg, ok := zhMessages[key]; ok {
			return msg
		}
	case LangEN:
		if msg, ok := enMessages[key]; ok {
			return msg
		}
	}
	return ""
}
