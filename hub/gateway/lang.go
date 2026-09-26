package gateway

import (
	"os"
	"strings"
)

// Lang identifies the hub Web UI language (FEATURE-559).
type Lang string

const (
	// LangZH is Chinese, the default UI language.
	LangZH Lang = "zh"
	// LangEN is English.
	LangEN Lang = "en"
)

// hubLangPlaceholder is substituted with the resolved language when the Web UI
// page is served (see WebUI.handleIndex).
const hubLangPlaceholder = "__HUB_LANG__"

// DetectLang resolves the hub Web UI language, mirroring co-shell's priority:
//  1. the --lang value (when supported)
//  2. the LANG environment variable
//  3. the LC_ALL environment variable
//  4. Chinese (zh) as the default
//
// Empty or unsupported values are skipped instead of failing, so the function
// always returns a usable language.
func DetectLang(flagValue string) Lang {
	for _, raw := range []string{flagValue, os.Getenv("LANG"), os.Getenv("LC_ALL")} {
		if lang, ok := normalizeLang(raw); ok {
			return lang
		}
	}
	return LangZH
}

// normalizeLang maps a raw language setting ("en", "en_US.UTF-8", "zh_CN") to a
// supported Lang. ok is false when the value is empty or unsupported.
func normalizeLang(raw string) (Lang, bool) {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return "", false
	}
	// Drop charset/modifier suffixes: "en_US.UTF-8" -> "en_us".
	if i := strings.IndexAny(v, ".@"); i >= 0 {
		v = v[:i]
	}
	switch {
	case strings.HasPrefix(v, "zh"):
		return LangZH, true
	case strings.HasPrefix(v, "en"):
		return LangEN, true
	}
	return "", false
}
