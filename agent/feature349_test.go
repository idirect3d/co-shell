// Author: L.Shuang
// Created: 2026-08-12
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

package agent

import (
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/i18n"
)

// FEATURE-349: judge 记忆化——已失败的 exit_strategy 回喂给二次判定模型。

func TestFEATURE349_RecordFailedStrategy_Basic(t *testing.T) {
	a := &Agent{}
	a.recordFailedLoopStrategyLocked("策略A：调用 read_file 读取 x.go")
	a.recordFailedLoopStrategyLocked("策略B：调用 search_files 搜索 y")

	got := a.failedLoopStrategiesSnapshot()
	if len(got) != 2 {
		t.Fatalf("expected 2 recorded strategies, got %d", len(got))
	}
	if got[0] != "策略A：调用 read_file 读取 x.go" || got[1] != "策略B：调用 search_files 搜索 y" {
		t.Errorf("unexpected order/content: %v", got)
	}
}

func TestFEATURE349_RecordFailedStrategy_EmptySkipped(t *testing.T) {
	a := &Agent{}
	a.recordFailedLoopStrategyLocked("")
	a.recordFailedLoopStrategyLocked("   ")
	if got := a.failedLoopStrategiesSnapshot(); len(got) != 0 {
		t.Fatalf("empty strategies must be skipped, got %v", got)
	}
}

func TestFEATURE349_RecordFailedStrategy_ConsecutiveDupCollapsed(t *testing.T) {
	a := &Agent{}
	a.recordFailedLoopStrategyLocked("同一策略")
	a.recordFailedLoopStrategyLocked("同一策略")
	a.recordFailedLoopStrategyLocked("不同策略")
	a.recordFailedLoopStrategyLocked("同一策略") // 非连续重复：允许（可能换了一轮又失败）

	got := a.failedLoopStrategiesSnapshot()
	if len(got) != 3 {
		t.Fatalf("expected 3 (consecutive dup collapsed), got %v", got)
	}
}

func TestFEATURE349_RecordFailedStrategy_Capped(t *testing.T) {
	a := &Agent{}
	for _, s := range []string{"s1", "s2", "s3", "s4", "s5"} {
		a.recordFailedLoopStrategyLocked(s)
	}
	got := a.failedLoopStrategiesSnapshot()
	if len(got) != loopFailedStrategiesMax {
		t.Fatalf("expected cap %d, got %d", loopFailedStrategiesMax, len(got))
	}
	if got[0] != "s3" || got[2] != "s5" {
		t.Errorf("expected most recent kept (s3..s5), got %v", got)
	}
}

func TestFEATURE349_BuildFailedStrategiesText_None(t *testing.T) {
	a := &Agent{}
	text := a.buildFailedStrategiesText()
	want := i18n.T(i18n.KeyLoopFailedStrategiesNone)
	if text != want {
		t.Errorf("expected none-text %q, got %q", want, text)
	}
	if strings.TrimSpace(text) == "" {
		t.Error("none-text must not be empty")
	}
}

func TestFEATURE349_BuildFailedStrategiesText_Numbered(t *testing.T) {
	a := &Agent{}
	a.recordFailedLoopStrategyLocked("策略一")
	a.recordFailedLoopStrategyLocked("策略二")
	text := a.buildFailedStrategiesText()
	// 每条策略必须用带序号的 BEGIN/END 哨兵行包裹，且内容原样保留。
	for _, want := range []string{
		"[FAILED-STRATEGY #1 BEGIN]\n策略一\n[FAILED-STRATEGY #1 END]",
		"[FAILED-STRATEGY #2 BEGIN]\n策略二\n[FAILED-STRATEGY #2 END]",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("expected sentinel-wrapped entry %q, got %q", want, text)
		}
	}
}

// 哨兵分隔符必须与策略内容可区分：即使策略内容里含编号样式文本，
// BEGIN/END 标记行仍然独占一行且格式唯一。
func TestFEATURE349_FailedStrategyDelimitersNoCollision(t *testing.T) {
	a := &Agent{}
	a.recordFailedLoopStrategyLocked("1. 先调用 search_files 搜索 [FAILED-STRATEGY") // 内容含相似片段
	text := a.buildFailedStrategiesText()
	lines := strings.Split(text, "\n")
	if lines[0] != "[FAILED-STRATEGY #1 BEGIN]" {
		t.Errorf("first line must be the BEGIN sentinel, got %q", lines[0])
	}
	if lines[len(lines)-1] != "[FAILED-STRATEGY #1 END]" {
		t.Errorf("last line must be the END sentinel, got %q", lines[len(lines)-1])
	}
	if len(lines) != 3 {
		t.Errorf("expected exactly 3 lines (BEGIN/content/END), got %d: %q", len(lines), text)
	}
}

// buildLoopJudgeUserPrompt 必须把失败策略填进 {FAILED_STRATEGIES} 占位符，
// 且模板中不残留占位符。
func TestFEATURE349_JudgePromptContainsFailedStrategies(t *testing.T) {
	a := &Agent{}
	a.recordFailedLoopStrategyLocked("失败策略X：调用 read_file 读取 config.go")

	prompt := a.buildLoopJudgeUserPrompt("计划文本", "疑似循环内容")
	if strings.Contains(prompt, "{FAILED_STRATEGIES}") {
		t.Error("placeholder {FAILED_STRATEGIES} was not substituted")
	}
	if !strings.Contains(prompt, "失败策略X：调用 read_file 读取 config.go") {
		t.Errorf("judge prompt should contain the failed strategy, got:\n%s", prompt)
	}
}

func TestFEATURE349_JudgePromptFirstJudgmentShowsNone(t *testing.T) {
	a := &Agent{}
	prompt := a.buildLoopJudgeUserPrompt("计划文本", "疑似循环内容")
	if strings.Contains(prompt, "{FAILED_STRATEGIES}") {
		t.Error("placeholder {FAILED_STRATEGIES} was not substituted")
	}
	if !strings.Contains(prompt, i18n.T(i18n.KeyLoopFailedStrategiesNone)) {
		t.Errorf("first judgment should show the none-text, got:\n%s", prompt)
	}
}

// 中英文模板都必须包含 {FAILED_STRATEGIES} 占位符（防止只改了一套语言）。
func TestFEATURE349_BothLangTemplatesHavePlaceholder(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		i18n.SetLang(lang)
		tpl := i18n.T(i18n.KeyLoopJudgeUserPrompt)
		if !strings.Contains(tpl, "{FAILED_STRATEGIES}") {
			t.Errorf("lang %s: KeyLoopJudgeUserPrompt missing {FAILED_STRATEGIES} placeholder", lang)
		}
		if i18n.T(i18n.KeyLoopFailedStrategiesNone) == "" {
			t.Errorf("lang %s: KeyLoopFailedStrategiesNone is empty", lang)
		}
	}
	i18n.SetLang("zh")
}
