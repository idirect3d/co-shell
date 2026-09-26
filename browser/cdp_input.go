// Input domain helpers: mouse events (move/hover/click with button and click
// count) and keyboard input (text insertion plus named keys and shortcuts).
//
// Text goes through Input.insertText so non-ASCII characters (CJK, emoji) are
// delivered correctly; named keys and modifier combinations are dispatched as
// real key events with proper virtual key codes and modifier bits.

package browser

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/idirect3d/co-shell/log"
)

// Mouse button names accepted by NormalizeMouseButton.
const (
	MouseButtonLeft   = "left"
	MouseButtonRight  = "right"
	MouseButtonMiddle = "middle"
)

// Modifier bit flags as defined by the CDP Input domain.
const (
	ModifierAlt   = 1
	ModifierCtrl  = 2
	ModifierMeta  = 4
	ModifierShift = 8
)

// keyDef describes one key for Input.dispatchKeyEvent.
type keyDef struct {
	Key  string // DOM "key" value, e.g. "Enter"
	Code string // DOM "code" value, e.g. "KeyA" (empty when unknown)
	VK   int    // Windows virtual key code
	Text string // text the key produces ("\r" for Enter); empty for modifiers
}

// namedKeys maps lower-cased key names to their CDP key definition.
var namedKeys = map[string]keyDef{
	"enter":      {Key: "Enter", Code: "Enter", VK: 13, Text: "\r"},
	"return":     {Key: "Enter", Code: "Enter", VK: 13, Text: "\r"},
	"tab":        {Key: "Tab", Code: "Tab", VK: 9, Text: "\t"},
	"escape":     {Key: "Escape", Code: "Escape", VK: 27},
	"esc":        {Key: "Escape", Code: "Escape", VK: 27},
	"backspace":  {Key: "Backspace", Code: "Backspace", VK: 8},
	"delete":     {Key: "Delete", Code: "Delete", VK: 46},
	"del":        {Key: "Delete", Code: "Delete", VK: 46},
	"arrowup":    {Key: "ArrowUp", Code: "ArrowUp", VK: 38},
	"arrowdown":  {Key: "ArrowDown", Code: "ArrowDown", VK: 40},
	"arrowleft":  {Key: "ArrowLeft", Code: "ArrowLeft", VK: 37},
	"arrowright": {Key: "ArrowRight", Code: "ArrowRight", VK: 39},
	"up":         {Key: "ArrowUp", Code: "ArrowUp", VK: 38},
	"down":       {Key: "ArrowDown", Code: "ArrowDown", VK: 40},
	"left":       {Key: "ArrowLeft", Code: "ArrowLeft", VK: 37},
	"right":      {Key: "ArrowRight", Code: "ArrowRight", VK: 39},
	"home":       {Key: "Home", Code: "Home", VK: 36},
	"end":        {Key: "End", Code: "End", VK: 35},
	"pageup":     {Key: "PageUp", Code: "PageUp", VK: 33},
	"pagedown":   {Key: "PageDown", Code: "PageDown", VK: 34},
	"space":      {Key: " ", Code: "Space", VK: 32, Text: " "},
}

// NamedKey looks up a named key (case-insensitive).
func NamedKey(name string) (keyDef, bool) {
	def, ok := namedKeys[strings.ToLower(strings.TrimSpace(name))]
	return def, ok
}

// modifierBit maps a modifier name to its CDP bit flag.
func modifierBit(name string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "alt", "option":
		return ModifierAlt, true
	case "ctrl", "control":
		return ModifierCtrl, true
	case "meta", "cmd", "command", "win", "super":
		return ModifierMeta, true
	case "shift":
		return ModifierShift, true
	}
	return 0, false
}

// ParseKeySpec parses a key specification such as "Enter" or "Ctrl+A" into a
// key definition plus the CDP modifier bit mask.
func ParseKeySpec(spec string) (keyDef, int, error) {
	parts := strings.Split(spec, "+")
	name := strings.TrimSpace(parts[len(parts)-1])
	if name == "" {
		return keyDef{}, 0, fmt.Errorf("empty key name in %q", spec)
	}

	mods := 0
	for _, part := range parts[:len(parts)-1] {
		bit, ok := modifierBit(part)
		if !ok {
			return keyDef{}, 0, fmt.Errorf("unknown modifier %q in %q", strings.TrimSpace(part), spec)
		}
		mods |= bit
	}

	if def, ok := NamedKey(name); ok {
		return def, mods, nil
	}

	r, size := utf8.DecodeRuneInString(name)
	if size != len(name) || r <= ' ' || r == 0x7f || r > unicode.MaxASCII {
		return keyDef{}, 0, fmt.Errorf("unexpected character %q: non-ASCII text must be inserted as text, not pressed as a key", name)
	}
	def := keyDef{Key: string(r), VK: int(unicode.ToUpper(r))}
	switch {
	case r >= 'a' && r <= 'z':
		def.Code = "Key" + string(unicode.ToUpper(r))
		if mods&ModifierShift != 0 {
			def.Key = string(unicode.ToUpper(r))
		}
	case r >= 'A' && r <= 'Z':
		def.Code = "Key" + string(r)
	case r >= '0' && r <= '9':
		def.Code = "Digit" + string(r)
	}
	// Shortcuts (Ctrl/Cmd/Alt + key) must not carry text — the page decides.
	if mods&(ModifierCtrl|ModifierAlt|ModifierMeta) == 0 {
		def.Text = string(r)
	}
	return def, mods, nil
}

// NormalizeShortcutModifiers adapts modifier bits to the platform shortcut
// convention: on darwin the Cmd key (Meta) is used where other platforms use
// Ctrl, so "Ctrl+A" means "select all" on every platform.
func NormalizeShortcutModifiers(modifiers int, goos string) int {
	if goos == "darwin" && modifiers&ModifierCtrl != 0 {
		return modifiers&^ModifierCtrl | ModifierMeta
	}
	return modifiers
}

// InsertText inserts text at the current caret position using
// Input.insertText. It works for any Unicode content, including CJK and emoji,
// which cannot be expressed with virtual key codes.
func (c *CDPClient) InsertText(ctx context.Context, text string) error {
	if text == "" {
		return nil
	}
	if _, err := c.Call(ctx, "Input.insertText", map[string]interface{}{"text": text}); err != nil {
		return fmt.Errorf("cannot insert text: %w", err)
	}
	return nil
}

// pressKey dispatches one keyDown/keyUp pair for def with the given modifiers.
func (c *CDPClient) pressKey(ctx context.Context, def keyDef, modifiers int) error {
	down := map[string]interface{}{
		"type":                  "rawKeyDown",
		"windowsVirtualKeyCode": def.VK,
		"nativeVirtualKeyCode":  def.VK,
		"key":                   def.Key,
		"modifiers":             modifiers,
	}
	if def.Code != "" {
		down["code"] = def.Code
	}
	if def.Text != "" {
		down["type"] = "keyDown"
		down["text"] = def.Text
	}
	if _, err := c.Call(ctx, "Input.dispatchKeyEvent", down); err != nil {
		return err
	}

	up := map[string]interface{}{
		"type":                  "keyUp",
		"windowsVirtualKeyCode": def.VK,
		"nativeVirtualKeyCode":  def.VK,
		"key":                   def.Key,
		"modifiers":             modifiers,
	}
	if def.Code != "" {
		up["code"] = def.Code
	}
	_, err := c.Call(ctx, "Input.dispatchKeyEvent", up)
	return err
}

// PressKeys presses one key or a comma-separated sequence of keys, e.g.
// "Enter" or "Ctrl+A,Backspace".
func (c *CDPClient) PressKeys(ctx context.Context, spec string) error {
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		def, mods, err := ParseKeySpec(part)
		if err != nil {
			return err
		}
		if err := c.pressKey(ctx, def, NormalizeShortcutModifiers(mods, runtime.GOOS)); err != nil {
			return fmt.Errorf("cannot press %q: %w", part, err)
		}
	}
	return nil
}

// typeToken is a piece of Type input: either literal text or a key to press.
type typeToken struct {
	text  string
	key   string
	isKey bool
}

// tokenizeTypeText splits text into literal runs and key presses. "\n" and
// "\t" become Enter/Tab, and "{...}" becomes a key press when its content is a
// valid key spec (e.g. "{Enter}", "{Ctrl+A}"); otherwise the braces are kept
// verbatim.
func tokenizeTypeText(text string) []typeToken {
	var tokens []typeToken
	var buf strings.Builder

	flush := func() {
		if buf.Len() > 0 {
			tokens = append(tokens, typeToken{text: buf.String()})
			buf.Reset()
		}
	}

	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		switch {
		case ch == '\n':
			flush()
			tokens = append(tokens, typeToken{key: "Enter", isKey: true})
		case ch == '\t':
			flush()
			tokens = append(tokens, typeToken{key: "Tab", isKey: true})
		case ch == '{':
			end := -1
			for j := i + 1; j < len(runes); j++ {
				if runes[j] == '}' {
					end = j
					break
				}
			}
			if end > i+1 {
				candidate := string(runes[i+1 : end])
				if _, _, err := ParseKeySpec(candidate); err == nil {
					flush()
					tokens = append(tokens, typeToken{key: candidate, isKey: true})
					i = end
					continue
				}
			}
			buf.WriteRune(ch)
		default:
			buf.WriteRune(ch)
		}
	}
	flush()
	return tokens
}

// TypeText types text into the focused element: literal runs are inserted with
// Input.insertText (Unicode-safe) and key tokens are pressed as real keys.
func (c *CDPClient) TypeText(ctx context.Context, text string) error {
	for _, tok := range tokenizeTypeText(text) {
		if tok.isKey {
			if err := c.PressKeys(ctx, tok.key); err != nil {
				return err
			}
			continue
		}
		if err := c.InsertText(ctx, tok.text); err != nil {
			return err
		}
	}
	return nil
}

// NormalizeMouseButton validates a mouse button name; an empty name defaults
// to the left button (the historical behaviour).
func NormalizeMouseButton(button string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(button)) {
	case "", MouseButtonLeft:
		return MouseButtonLeft, nil
	case MouseButtonRight:
		return MouseButtonRight, nil
	case MouseButtonMiddle:
		return MouseButtonMiddle, nil
	}
	return "", fmt.Errorf("unsupported mouse button %q (use left, right or middle)", button)
}

// mouseButtonMask returns the "buttons" bit mask for a pressed button.
func mouseButtonMask(button string) int {
	switch button {
	case MouseButtonRight:
		return 2
	case MouseButtonMiddle:
		return 4
	default:
		return 1
	}
}

// MouseMove moves the mouse pointer to the given coordinates without pressing
// any button. Pages that reveal menus on hover need this before a click.
func (c *CDPClient) MouseMove(ctx context.Context, x, y float64) error {
	_, err := c.Call(ctx, "Input.dispatchMouseEvent", map[string]interface{}{
		"type":    "mouseMoved",
		"x":       x,
		"y":       y,
		"button":  "none",
		"buttons": 0,
	})
	return err
}

// Hover moves the mouse pointer over the given coordinates (alias of MouseMove
// with hover semantics).
func (c *CDPClient) Hover(ctx context.Context, x, y float64) error {
	return c.MouseMove(ctx, x, y)
}

// ClickEx performs a click with an explicit mouse button and click count
// (2 = double click). The pointer is moved first so hover-dependent targets
// react; each press/release pair carries an increasing clickCount.
func (c *CDPClient) ClickEx(ctx context.Context, x, y float64, button string, clickCount int) error {
	btn, err := NormalizeMouseButton(button)
	if err != nil {
		return err
	}
	if clickCount < 1 {
		clickCount = 1
	}
	if err := c.MouseMove(ctx, x, y); err != nil {
		return err
	}

	for i := 1; i <= clickCount; i++ {
		if _, err := c.Call(ctx, "Input.dispatchMouseEvent", map[string]interface{}{
			"type":       "mousePressed",
			"x":          x,
			"y":          y,
			"button":     btn,
			"buttons":    mouseButtonMask(btn),
			"clickCount": i,
		}); err != nil {
			return err
		}
		if _, err := c.Call(ctx, "Input.dispatchMouseEvent", map[string]interface{}{
			"type":       "mouseReleased",
			"x":          x,
			"y":          y,
			"button":     btn,
			"buttons":    0,
			"clickCount": i,
		}); err != nil {
			return err
		}
	}
	return nil
}

// ScrollByWheel scrolls the page with a real mouse wheel event dispatched at
// the viewport centre. It falls back to window.scrollBy when the wheel event
// cannot be delivered (e.g. no viewport metrics available).
func (c *CDPClient) ScrollByWheel(ctx context.Context, deltaX, deltaY float64) error {
	width, height, err := c.GetViewportSize(ctx)
	if err == nil && width > 0 && height > 0 {
		_, wheelErr := c.Call(ctx, "Input.dispatchMouseEvent", map[string]interface{}{
			"type":   "mouseWheel",
			"x":      width / 2,
			"y":      height / 2,
			"deltaX": deltaX,
			"deltaY": deltaY,
		})
		if wheelErr == nil {
			return nil
		}
		log.Debug("mouseWheel scroll failed, falling back to window.scrollBy: %v", wheelErr)
	}

	if _, evalErr := c.Evaluate(ctx, fmt.Sprintf("window.scrollBy(%f, %f)", deltaX, deltaY)); evalErr != nil {
		return fmt.Errorf("cannot scroll page: %w", evalErr)
	}
	return nil
}
