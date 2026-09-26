package gateway

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// FEATURE-559: hub 抽屉文案国际化（分区标题统一为 Agents / Agent 聊天室）。
// These tests cover the A group of use-case/FEATURE-559/FEATURE-559-UC-0001.md.

func feat559ReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

const feat559StaticPath = "webui_static.go"

// enDictKeys returns the keys declared in the EN dictionary of the static page.
func enDictKeys(t *testing.T, page string) map[string]bool {
	t.Helper()
	start := strings.Index(page, "var EN = {")
	if start < 0 {
		t.Fatal("EN dictionary not found in webui_static.go")
	}
	rest := page[start:]
	end := strings.Index(rest, "\n  };")
	if end < 0 {
		t.Fatal("EN dictionary is not terminated by \"\\n  };\"")
	}
	body := rest[:end]
	keys := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^\s{4}([A-Za-z0-9_]+):`).FindAllStringSubmatch(body, -1) {
		keys[m[1]] = true
	}
	if len(keys) == 0 {
		t.Fatal("EN dictionary has no entries (indentation changed?)")
	}
	return keys
}

// A-001: --lang wins over the environment.
func TestFeat559DetectLangFlagWins(t *testing.T) {
	t.Setenv("LANG", "zh_CN.UTF-8")
	t.Setenv("LC_ALL", "zh_CN.UTF-8")
	if got := DetectLang("en"); got != LangEN {
		t.Fatalf("DetectLang(\"en\") = %q, want %q", got, LangEN)
	}
	t.Setenv("LANG", "en_US.UTF-8")
	t.Setenv("LC_ALL", "en_US.UTF-8")
	if got := DetectLang("zh"); got != LangZH {
		t.Fatalf("DetectLang(\"zh\") = %q, want %q", got, LangZH)
	}
}

// A-002 / A-003: environment detection, unknowns and empty values fall back to zh.
func TestFeat559DetectLangFromEnv(t *testing.T) {
	cases := []struct {
		name  string
		lang  string
		lcAll string
		want  Lang
	}{
		{"lang zh", "zh_CN.UTF-8", "", LangZH},
		{"lang en", "en_US.UTF-8", "", LangEN},
		{"bare code", "en", "", LangEN},
		{"lc_all fallback", "", "en_GB.UTF-8", LangEN},
		{"unknown locale", "fr_FR.UTF-8", "", LangZH},
		{"empty env", "", "", LangZH},
		{"modifier suffix", "en_US@euro", "", LangEN},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("LANG", c.lang)
			t.Setenv("LC_ALL", c.lcAll)
			if got := DetectLang(""); got != c.want {
				t.Fatalf("DetectLang(\"\") with LANG=%q LC_ALL=%q = %q, want %q",
					c.lang, c.lcAll, got, c.want)
			}
		})
	}
}

// A-003: an unsupported --lang is skipped (never an error), so the environment
// still decides.
func TestFeat559UnsupportedFlagFallsBackToEnv(t *testing.T) {
	t.Setenv("LANG", "en_US.UTF-8")
	t.Setenv("LC_ALL", "")
	if got := DetectLang("fr"); got != LangEN {
		t.Fatalf("DetectLang(\"fr\") = %q, want %q (env fallback)", got, LangEN)
	}
}

// A-004: the served page carries the resolved language and no placeholder.
func TestFeat559HandleIndexInjectsLanguage(t *testing.T) {
	t.Setenv("LANG", "zh_CN.UTF-8")
	t.Setenv("LC_ALL", "")
	cases := []struct {
		name string
		lang string
		want string
	}{
		{"default zh", "", `var HUB_LANG = "zh";`},
		{"explicit en", "en", `var HUB_LANG = "en";`},
		{"unknown falls back", "fr", `var HUB_LANG = "zh";`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := &WebUI{lang: DetectLang(c.lang)}
			rec := httptest.NewRecorder()
			w.handleIndex(rec, httptest.NewRequest("GET", "/", nil))
			body := rec.Body.String()
			if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
				t.Errorf("Content-Type = %q", ct)
			}
			if !strings.Contains(body, c.want) {
				t.Errorf("page does not contain %q", c.want)
			}
			if strings.Contains(body, hubLangPlaceholder) {
				t.Errorf("language placeholder %q was left unsubstituted", hubLangPlaceholder)
			}
		})
	}
}

// A-005: every key referenced by the markup or by t() has an English entry, and
// no entry is empty.
func TestFeat559EnglishDictionaryCoversPage(t *testing.T) {
	page := feat559ReadFile(t, feat559StaticPath)
	en := enDictKeys(t, page)

	used := map[string]bool{}
	for _, m := range regexp.MustCompile(`data-i18n(?:-title|-ph)?="([^"]+)"`).FindAllStringSubmatch(page, -1) {
		used[m[1]] = true
	}
	for _, m := range regexp.MustCompile(`\bt\('([A-Za-z0-9_]+)'`).FindAllStringSubmatch(page, -1) {
		used[m[1]] = true
	}
	if len(used) < 50 {
		t.Fatalf("only %d i18n keys found; expected the drawer strings to be keyed", len(used))
	}
	for k := range used {
		if !en[k] {
			t.Errorf("key %q is used but missing from the EN dictionary", k)
		}
	}
	if regexp.MustCompile(`(?m)^\s{4}[A-Za-z0-9_]+: '',\s*$`).MatchString(page) {
		t.Error("EN dictionary contains an empty value")
	}
}

// A-006: both pane titles go through the dictionary — English is plural for both
// (Agents / Chats), Chinese keeps the Agent term (Agents / Agent 聊天室).
func TestFeat559PaneTitles(t *testing.T) {
	page := feat559ReadFile(t, feat559StaticPath)

	for _, want := range []string{
		`<span data-i18n="paneAgents">Agents</span>`,
		`<span data-i18n="paneChats">Agent 聊天室</span>`,
		`paneAgents: 'Agents',`,
		`paneChats: 'Chats',`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("static page does not contain %q", want)
		}
	}
	// The old inline singular title must be gone.
	if strings.Contains(page, `id="chatHead">Chat<`) {
		t.Error("chat pane still hardcodes the singular title \"Chat\"")
	}
	if strings.Contains(page, `id="agentsHead">Agents<`) {
		t.Error("agents pane still hardcodes its title outside the dictionary")
	}
}

// A-007: no user-visible literal is left un-translated — every remaining CJK
// literal outside comments is either a data-i18n* fallback or passed to t().
func TestFeat559NoUntranslatedLiterals(t *testing.T) {
	page := feat559ReadFile(t, feat559StaticPath)
	stripComments := func(s string) string {
		s = regexp.MustCompile(`(?s)<!--.*?-->`).ReplaceAllString(s, "")
		return regexp.MustCompile(`(?m)//[^\n]*`).ReplaceAllString(s, "")
	}
	cjk := regexp.MustCompile(`[\p{Han}]`)
	for i, line := range strings.Split(stripComments(page), "\n") {
		if !cjk.MatchString(line) {
			continue
		}
		if strings.Contains(line, "data-i18n") || strings.Contains(line, "t('") {
			continue
		}
		t.Errorf("line %d carries an untranslated literal: %s", i+1, strings.TrimSpace(line))
	}
}

// A-009 (decision: out of scope): backend client-facing errors stay English.
func TestFeat559BackendErrorsRemainEnglish(t *testing.T) {
	for _, path := range []string{"webui.go", "webui_detect.go"} {
		src := feat559ReadFile(t, path)
		for _, want := range []string{`"bad request"`, `"agent not found"`} {
			if path == "webui_detect.go" {
				want = `"path required"`
			}
			if !strings.Contains(src, want) {
				t.Errorf("%s: expected the backend error %s to remain unchanged", path, want)
			}
		}
	}
}

// A-010: co-shell and co-shell-hub share the same version and build number.
func TestFeat559VersionAndBuildInSync(t *testing.T) {
	root := feat559ReadFile(t, filepath.Join("..", "..", "main.go"))
	hubMain := feat559ReadFile(t, filepath.Join("..", "..", "cmd", "co-shell-hub", "main.go"))
	match := func(src, pattern, label string) string {
		m := regexp.MustCompile(pattern).FindStringSubmatch(src)
		if m == nil {
			t.Fatalf("%s not found in %s", label, src)
		}
		return m[1]
	}
	coreVer := match(root, `(?m)^const version = "([^"]+)"`, "version")
	coreBuild := match(root, `(?m)^const build = "([^"]+)"`, "build")
	hubVer := match(hubMain, `hubVersion = "([^"]+)"`, "hubVersion")
	hubBuild := match(hubMain, `hubBuild\s+= "([^"]+)"`, "hubBuild")

	if coreVer != hubVer {
		t.Errorf("version mismatch: co-shell %s != co-shell-hub %s", coreVer, hubVer)
	}
	if coreBuild != hubBuild {
		t.Errorf("build mismatch: co-shell %s != co-shell-hub %s", coreBuild, hubBuild)
	}
}
