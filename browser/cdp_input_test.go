// Unit tests for the Input-domain helpers (key mapping, shortcuts, text
// insertion and mouse buttons) plus dialog policy parsing.
//
// Use-case: use-case/FEATURE-556/FEATURE-556-UC-0019.md (group A).

package browser

import (
	"reflect"
	"testing"
)

// TestParseKeySpecNamedKeys covers A-006: every named key must map to the
// virtual key code, DOM code and DOM key the browser expects.
func TestParseKeySpecNamedKeys(t *testing.T) {
	cases := []struct {
		spec string
		vk   int
		code string
		key  string
		text string
	}{
		{"Enter", 13, "Enter", "Enter", "\r"},
		{"Tab", 9, "Tab", "Tab", "\t"},
		{"Escape", 27, "Escape", "Escape", ""},
		{"esc", 27, "Escape", "Escape", ""},
		{"Backspace", 8, "Backspace", "Backspace", ""},
		{"Delete", 46, "Delete", "Delete", ""},
		{"ArrowUp", 38, "ArrowUp", "ArrowUp", ""},
		{"ArrowDown", 40, "ArrowDown", "ArrowDown", ""},
		{"ArrowLeft", 37, "ArrowLeft", "ArrowLeft", ""},
		{"ArrowRight", 39, "ArrowRight", "ArrowRight", ""},
		{"Home", 36, "Home", "Home", ""},
		{"End", 35, "End", "End", ""},
		{"PageUp", 33, "PageUp", "PageUp", ""},
		{"PageDown", 34, "PageDown", "PageDown", ""},
	}

	for _, tc := range cases {
		def, mods, err := ParseKeySpec(tc.spec)
		if err != nil {
			t.Fatalf("ParseKeySpec(%q) failed: %v", tc.spec, err)
		}
		if mods != 0 {
			t.Errorf("ParseKeySpec(%q): modifiers = %d, want 0", tc.spec, mods)
		}
		if def.VK != tc.vk || def.Code != tc.code || def.Key != tc.key || def.Text != tc.text {
			t.Errorf("ParseKeySpec(%q) = %+v, want vk=%d code=%s key=%s text=%q",
				tc.spec, def, tc.vk, tc.code, tc.key, tc.text)
		}
	}
}

// TestParseKeySpecCharacters checks single-character keys.
func TestParseKeySpecCharacters(t *testing.T) {
	def, mods, err := ParseKeySpec("a")
	if err != nil {
		t.Fatalf("ParseKeySpec(a) failed: %v", err)
	}
	if def.VK != 65 || def.Code != "KeyA" || def.Key != "a" || def.Text != "a" || mods != 0 {
		t.Errorf("ParseKeySpec(a) = %+v mods=%d, want vk=65 code=KeyA key=a text=a", def, mods)
	}

	def, _, err = ParseKeySpec("1")
	if err != nil {
		t.Fatalf("ParseKeySpec(1) failed: %v", err)
	}
	if def.VK != 49 || def.Code != "Digit1" || def.Key != "1" || def.Text != "1" {
		t.Errorf("ParseKeySpec(1) = %+v, want vk=49 code=Digit1", def)
	}
}

// TestParseKeySpecShortcutModifiers covers A-007: Ctrl combinations carry the
// Ctrl bit (2) on non-darwin platforms and the Meta/Cmd bit (4) on darwin.
func TestParseKeySpecShortcutModifiers(t *testing.T) {
	for _, spec := range []string{"Ctrl+A", "Ctrl+C", "Ctrl+V", "Ctrl+X", "Ctrl+Z"} {
		def, mods, err := ParseKeySpec(spec)
		if err != nil {
			t.Fatalf("ParseKeySpec(%q) failed: %v", spec, err)
		}
		if mods != ModifierCtrl {
			t.Errorf("ParseKeySpec(%q): modifiers = %d, want %d", spec, mods, ModifierCtrl)
		}
		if def.Text != "" {
			t.Errorf("ParseKeySpec(%q): text = %q, want empty for a shortcut", spec, def.Text)
		}
	}

	cases := []struct {
		mods int
		goos string
		want int
	}{
		{ModifierCtrl, "darwin", ModifierMeta},
		{ModifierCtrl, "linux", ModifierCtrl},
		{ModifierCtrl, "windows", ModifierCtrl},
		{ModifierMeta, "darwin", ModifierMeta},
		{ModifierCtrl | ModifierShift, "darwin", ModifierMeta | ModifierShift},
		{ModifierShift, "darwin", ModifierShift},
	}
	for _, tc := range cases {
		if got := NormalizeShortcutModifiers(tc.mods, tc.goos); got != tc.want {
			t.Errorf("NormalizeShortcutModifiers(%d, %s) = %d, want %d", tc.mods, tc.goos, got, tc.want)
		}
	}
}

// TestParseKeySpecErrors ensures invalid specs are rejected instead of
// silently producing a bogus key event.
func TestParseKeySpecErrors(t *testing.T) {
	for _, spec := range []string{"", "Foo", "Hyper+A", "Ctrl+", "😀"} {
		if _, _, err := ParseKeySpec(spec); err == nil {
			t.Errorf("ParseKeySpec(%q) should fail", spec)
		}
	}
}

// TestTokenizeTypeText covers A-008: Unicode text stays literal, while "\n",
// "\t" and {Key} tokens become real key presses.
func TestTokenizeTypeText(t *testing.T) {
	cases := []struct {
		in   string
		want []typeToken
	}{
		{"你好，世界🙂", []typeToken{{text: "你好，世界🙂"}}},
		{"abc\n", []typeToken{{text: "abc"}, {key: "Enter", isKey: true}}},
		{"a{Tab}b", []typeToken{{text: "a"}, {key: "Tab", isKey: true}, {text: "b"}}},
		{"{Ctrl+A}", []typeToken{{key: "Ctrl+A", isKey: true}}},
		{"x{Foo}y", []typeToken{{text: "x{Foo}y"}}},
		{"", nil},
	}

	for _, tc := range cases {
		got := tokenizeTypeText(tc.in)
		if len(got) == 0 && len(tc.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("tokenizeTypeText(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
	}
}

// TestNormalizeMouseButton ensures the historical default (left) is kept and
// unknown buttons are rejected instead of clicking the wrong one.
func TestNormalizeMouseButton(t *testing.T) {
	ok := map[string]string{
		"":        MouseButtonLeft,
		"left":    MouseButtonLeft,
		"LEFT":    MouseButtonLeft,
		" right ": MouseButtonRight,
		"middle":  MouseButtonMiddle,
	}
	for in, want := range ok {
		got, err := NormalizeMouseButton(in)
		if err != nil {
			t.Errorf("NormalizeMouseButton(%q) failed: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("NormalizeMouseButton(%q) = %q, want %q", in, got, want)
		}
	}

	for _, in := range []string{"wheel", "back"} {
		if _, err := NormalizeMouseButton(in); err == nil {
			t.Errorf("NormalizeMouseButton(%q) should fail", in)
		}
	}
}

// TestDialogPolicy covers A-012: policy parsing plus the accept/dismiss
// decision, including the "auto" rule that only accepts informative dialogs.
func TestDialogPolicy(t *testing.T) {
	cases := []struct {
		in   string
		want DialogPolicy
		ok   bool
	}{
		{"accept", DialogAccept, true},
		{"dismiss", DialogDismiss, true},
		{"auto", DialogAuto, true},
		{"", DialogAuto, false},
		{"bogus", DialogAuto, false},
	}
	for _, tc := range cases {
		got, ok := ParseDialogPolicy(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("ParseDialogPolicy(%q) = (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}

	for _, tc := range []struct {
		policy     DialogPolicy
		dialogType string
		want       bool
	}{
		{DialogAccept, "confirm", true},
		{DialogAccept, "beforeunload", true},
		{DialogDismiss, "alert", false},
		{DialogDismiss, "prompt", false},
		{DialogAuto, "alert", true},
		{DialogAuto, "prompt", true},
		{DialogAuto, "confirm", false},
		{DialogAuto, "beforeunload", false},
	} {
		mgr := &DialogManager{policy: tc.policy}
		if got := mgr.shouldAccept(tc.dialogType); got != tc.want {
			t.Errorf("policy %s: shouldAccept(%s) = %v, want %v", tc.policy, tc.dialogType, got, tc.want)
		}
	}
}
