// Author: L.Shuang
// Created: 2026-06-14
// Last Modified: 2026-07-01
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
// IMPLIED, BUT NOT INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package agent

import (
	"strings"
	"testing"
	"time"
)

// TestLoopDetector_RepeatLines verifies period detection:
// A,B,A,B (period=2, only 2 repetitions) does NOT trigger, but AAA does.
func TestLoopDetector_RepeatLines(t *testing.T) {
	ld := NewLoopDetector(3)

	lineA := "这是一行测试内容，用于生成重复文本文件。每行固定八十个字循环往复生成多行文本"
	lineB := "这是第二行测试内容，用于生成重复文本文件。每行固定八十个字循环往复生成多行文本"

	// A,B,A,B — period=2, only 2 repetitions, not enough to trigger threshold=3
	content := lineA + "\n" + lineB + "\n" + lineA + "\n" + lineB + "\n"
	err := ld.AddChunk(content, time.Now())
	if err != nil {
		t.Fatalf("A,B,A,B (2 reps) should NOT trigger, got: %v", err)
	}
	t.Log("correctly did not trigger on 2 repetitions of period-2")

	// Consecutive A,A,A — period=1, 3 repetitions — should trigger
	ld.Reset()
	content = lineA + "\n" + lineA + "\n" + lineA + "\n"
	err = ld.AddChunk(content, time.Now())
	if err == nil {
		t.Fatal("should trigger after 3 consecutive A's, but got nil")
	}
	t.Logf("consecutive loop detected as expected: %s", err.Error())
}

// TestLoopDetector_ShortLineFilter verifies that short lines participate
// in detection like any other line (no minLineLen filter).
func TestLoopDetector_ShortLineFilter(t *testing.T) {
	ld := NewLoopDetector(3)

	shortLine := "short"
	content := strings.Repeat(shortLine+"\n", 5)
	err := ld.AddChunk(content, time.Now())
	if err == nil {
		t.Fatal("short lines repeated 5 times consecutively should trigger threshold=3, but got nil")
	}
	t.Logf("short line loop detected as expected: %s", err.Error())
}

// TestLoopDetector_CrossChunk verifies cross-chunk consecutive line handling.
func TestLoopDetector_CrossChunk(t *testing.T) {
	ld := NewLoopDetector(3)

	lineA := "AAAAA这是一行测试内容AAAAA用于生成重复文本文件AAAAA"
	assertChunk := func(chunk string, expectTrigger bool) {
		err := ld.AddChunk(chunk, time.Now())
		if expectTrigger && err == nil {
			t.Fatal("expected trigger but got nil")
		}
		if !expectTrigger && err != nil {
			t.Fatalf("expected no trigger but got: %v", err)
		}
	}

	assertChunk(lineA+"\n"+lineA+"\n", false) // A=2, period=1, 2 reps < threshold 3
	assertChunk(lineA+"\n", true)             // A=3, period=1, 3 reps >= threshold 3
	t.Log("cross-chunk test passed")
}

// TestLoopDetector_ABCABC tests ABCABC repeating pattern (period=3).
func TestLoopDetector_UserReportedCase(t *testing.T) {
	ld := NewLoopDetector(5)

	lineA := "金融监管部门持续加强信息科技风险管理，要求金融机构建立健全信息安全管理体系，完善技术应急预案，确保业务连续性。"
	lineB := "网络安全防护方面，监管机构要求落实等级保护制度，加强关键信息基础设施安全保护，定期开展安全风险评估。"
	lineC := "数据安全管理方面，金融机构需严格执行数据分类分级制度，加强个人信息保护，防范数据泄露和滥用风险。"

	// ABCABC repeating 9 times — period=3, 9 repetitions > threshold 5
	var allLines []string
	for i := 0; i < 9; i++ {
		allLines = append(allLines, lineA, lineB, lineC)
	}

	chunk := strings.Join(allLines, "\n")
	err := ld.AddChunk(chunk, time.Now())
	if err == nil {
		t.Fatal("should trigger: ABCABC repeating loop pattern (period=3, 9 reps), but got nil")
	}
	ldErr, ok := err.(*LoopDetectedError)
	if !ok {
		t.Fatalf("expected *LoopDetectedError, got %T", err)
	}
	t.Logf("loop detected as expected: period=%d, repeated %d times (threshold=%d)",
		ldErr.period, ldErr.repeatCount, ldErr.threshold)
}

// TestLoopDetector_ScatteredRepetition tests that ABCDABCE pattern
// does NOT trigger — the last 3 lines A,B,C do not form a clean period
// because when scanning at the tail, A,B,C,E does not repeat.
func TestLoopDetector_ScatteredRepetition(t *testing.T) {
	ld := NewLoopDetector(3)

	lineA := "AAAAA这是一行测试内容AAAAA用于生成重复文本文件AAAAA"
	lineB := "BBBBB这是第二行测试内容BBBBB用于生成重复文本文件BBBBB"
	lineC := "CCCCC这是第三行测试内容CCCCC用于生成重复文本文件CCCCC"
	lineD := "DDDDD这是第四行测试内容DDDDD用于生成重复文本文件DDDDD"
	lineE := "EEEEE这是第五行测试内容EEEEE用于生成重复文本文件EEEEE"

	// Pattern: A,B,C,D,A,B,C,E
	// Tail: A,B,C,E — period=4 check: D vs E mismatch → no trigger
	chunk := lineA + "\n" + lineB + "\n" + lineC + "\n" + lineD + "\n" +
		lineA + "\n" + lineB + "\n" + lineC + "\n" + lineE + "\n"
	err := ld.AddChunk(chunk, time.Now())
	if err != nil {
		t.Fatalf("scattered ABCDABCE should NOT trigger, got: %v", err)
	}
	t.Log("correctly ignored scattered non-loop repetition")

	// True consecutive loop — should trigger
	ld.Reset()
	chunk = lineA + "\n" + lineA + "\n" + lineA + "\n"
	err = ld.AddChunk(chunk, time.Now())
	if err == nil {
		t.Fatal("should trigger after 3 consecutive A's, got nil")
	}
	t.Logf("consecutive loop detected: %s", err.Error())
}

// TestLoopDetector_StreamedChars simulates line-by-line same-line streaming.
func TestLoopDetector_StreamedChars(t *testing.T) {
	ld := NewLoopDetector(3)
	lineA := "金融监管部门持续加强信息科技风险管理，要求金融机构建立健全信息安全管理体系，完善技术应急预案，确保业务连续性。"

	for i := 0; i < 6; i++ {
		err := ld.AddChunk(lineA+"\n", time.Now())
		if i < 2 {
			if err != nil {
				t.Fatalf("should not trigger at iteration %d, got: %v", i, err)
			}
		} else {
			if err == nil {
				t.Fatalf("should trigger at iteration %d (count=%d), but got nil", i, i+1)
			}
			t.Logf("loop detected at iteration %d: %s", i, err.Error())
			return
		}
	}
	t.Fatal("expected trigger but never got one")
}

// TestLoopDetector_NewlineSeparateChunk tests "\n" as separate chunk token.
// ABCABC repeating pattern IS a loop and SHOULD trigger.
func TestLoopDetector_NewlineSeparateChunk(t *testing.T) {
	ld := NewLoopDetector(5)

	lineA := "金融监管部门持续加强信息科技风险管理，推动银行业金融机构建立健全信息系统治理架构和网络安全防护体系，确保关键信息基础设施安全稳定运行。"
	lineB := "网络安全法及配套法规体系不断完善，金融行业需严格落实等级保护制度，加强关键信息基础设施保护，建立网络安全监测预警和信息通报机制。"
	lineC := "数据安全管理要求日益严格，金融机构应建立健全数据分类分级保护制度，加强个人信息保护，完善数据全生命周期安全管理机制和应急响应体系。"

	// 8 cycles of A,B,C with \n as separate chunk — period=3, 8 reps > threshold 5
	for cycle := 0; cycle < 8; cycle++ {
		for _, line := range []string{lineA, lineB, lineC} {
			err := ld.AddChunk(line, time.Now())
			if err != nil {
				t.Fatalf("unexpected trigger after content at cycle %d: %v", cycle, err)
			}
			err = ld.AddChunk("\n", time.Now())
			if err != nil {
				t.Logf("loop detected at cycle %d: %s", cycle+1, err.Error())
				return
			}
		}
	}
	t.Fatal("should trigger after 8 cycles (each line appears 8 times >= threshold 5), but never detected")
}

// TestLoopDetector_NoNewlineAfterLine tests clean chunk boundaries.
func TestLoopDetector_NoNewlineAfterLine(t *testing.T) {
	ld := NewLoopDetector(3)
	lineA := "金融监管部门持续加强信息科技风险管理，要求金融机构建立健全信息安全管理体系，完善技术应急预案，确保业务连续性。"
	err := ld.AddChunk(lineA+"\n", time.Now())
	if err != nil {
		t.Fatalf("should not trigger at count=1, got: %v", err)
	}
}

// TestLoopDetector_MultiBlockChunk tests that A,A,B,B,A does NOT trigger.
func TestLoopDetector_MultiBlockChunk(t *testing.T) {
	ld := NewLoopDetector(3)
	lineA := "金融监管部门持续加强信息科技风险管理，要求金融机构建立健全信息安全管理体系，完善技术应急预案，确保业务连续性。"
	lineB := "网络安全防护方面，监管机构要求落实等级保护制度，加强关键信息基础设施安全保护，定期开展安全风险评估。"

	// Chunk: A,A,B,B
	// Buffer tail: A,B — period=1: A vs B mismatch; period=2: only 1 segment (B,B) matches
	// No clean period repeated >= 3 times → no trigger
	chunk := lineA + "\n" + lineA + "\n" + lineB + "\n" + lineB + "\n"
	err := ld.AddChunk(chunk, time.Now())
	if err != nil {
		t.Fatalf("should not trigger at count=2 for each line, got: %v", err)
	}
	err = ld.AddChunk(lineA+"\n", time.Now())
	if err != nil {
		t.Fatalf("should NOT trigger: A is scattered after B,B; got: %v", err)
	}
	t.Log("correctly ignored scattered A after B,B")

	// Consecutive A,A,A — should trigger
	ld.Reset()
	err = ld.AddChunk(lineA+"\n"+lineA+"\n"+lineA+"\n", time.Now())
	if err == nil {
		t.Fatal("should trigger after 3 consecutive A's, got nil")
	}
	t.Logf("consecutive loop detected: %s", err.Error())
}

// TestLoopDetector_ABACABAC tests ABACABAC period-4 repeating pattern.
// This is the case that M-max could NOT detect.
func TestLoopDetector_ABACABAC(t *testing.T) {
	ld := NewLoopDetector(3)

	lineA := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	lineB := "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
	lineC := "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"

	// ABACABAC — period=4, 2 repetitions, not enough for threshold=3
	content := lineA + "\n" + lineB + "\n" + lineA + "\n" + lineC + "\n" +
		lineA + "\n" + lineB + "\n" + lineA + "\n" + lineC + "\n"
	err := ld.AddChunk(content, time.Now())
	if err != nil {
		t.Fatalf("ABACABAC (2 reps) should NOT trigger at threshold=3, got: %v", err)
	}
	t.Log("correctly did not trigger on 2 repetitions of period-4")

	// ABACABACABAC — period=4, 3 repetitions, should trigger
	ld.Reset()
	content = lineA + "\n" + lineB + "\n" + lineA + "\n" + lineC + "\n" +
		lineA + "\n" + lineB + "\n" + lineA + "\n" + lineC + "\n" +
		lineA + "\n" + lineB + "\n" + lineA + "\n" + lineC + "\n"
	err = ld.AddChunk(content, time.Now())
	if err == nil {
		t.Fatal("ABACABACABAC (3 reps) should trigger at threshold=3, but got nil")
	}
	t.Logf("period-4 loop detected as expected: %s", err.Error())
}

// TestLoopDetector_Period10 tests a 10-line repeating paragraph pattern.
// This simulates the real-world case where the LLM repeats a block of
// ~10 lines of analysis output (p=10).
func TestLoopDetector_Period10(t *testing.T) {
	ld := NewLoopDetector(3)

	lines := []string{
		"实际上，从之前的分析中，我已经提取到了以下关键数据：",
		"- Treaty No: P5HP00032402",
		"- UwYear: 2025",
		"- A/C Period: 01Q",
		"- Currency: CNY",
		"- A/C Year: 2025",
		"- Broker: Beazley",
		"- Cedant: Asia-Pacific Property and Casualty Insurance Co., Ltd.",
		"- FilesName: P5HP00032402_TTY_58188413_BEI_Taiping Reinsurance (China) Company Limited._126664_Closing Instruction_8252025_No.pdf",
		"但是，我注意到之前的识别结果中，有些数据可能不准确。让我重新仔细分析PDF页面，确保提取所有有用的信息。",
	}

	// 2 repetitions (20 lines) → period=10, 2 reps < threshold=3, should NOT trigger
	var content string
	for i := 0; i < 2; i++ {
		for _, l := range lines {
			content += l + "\n"
		}
	}
	err := ld.AddChunk(content, time.Now())
	if err != nil {
		t.Fatalf("2 repetitions of period-10 should NOT trigger at threshold=3, got: %v", err)
	}
	t.Log("correctly did not trigger on 2 repetitions of period-10")

	// 3 repetitions (30 lines) → period=10, 3 reps >= threshold=3, SHOULD trigger
	ld.Reset()
	content = ""
	for i := 0; i < 3; i++ {
		for _, l := range lines {
			content += l + "\n"
		}
	}
	err = ld.AddChunk(content, time.Now())
	if err == nil {
		t.Fatal("3 repetitions of period-10 should trigger at threshold=3, but got nil")
	}
	t.Logf("period-10 loop detected as expected: %s", err.Error())
}

// TestLoopDetector_ABCDABCD tests ABCDABCD period-4 repeating pattern.
func TestLoopDetector_ABCDABCD(t *testing.T) {
	ld := NewLoopDetector(3)

	lineA := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	lineB := "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
	lineC := "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"
	lineD := "DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD"

	// ABCDABCD — period=4, 2 reps, not enough for threshold=3
	content := lineA + "\n" + lineB + "\n" + lineC + "\n" + lineD + "\n" +
		lineA + "\n" + lineB + "\n" + lineC + "\n" + lineD + "\n"
	err := ld.AddChunk(content, time.Now())
	if err != nil {
		t.Fatalf("ABCDABCD (2 reps) should NOT trigger at threshold=3, got: %v", err)
	}
	t.Log("correctly did not trigger on 2 repetitions of period-4")

	// ABCDABCDABCD — period=4, 3 reps, should trigger
	ld.Reset()
	content = lineA + "\n" + lineB + "\n" + lineC + "\n" + lineD + "\n" +
		lineA + "\n" + lineB + "\n" + lineC + "\n" + lineD + "\n" +
		lineA + "\n" + lineB + "\n" + lineC + "\n" + lineD + "\n"
	err = ld.AddChunk(content, time.Now())
	if err == nil {
		t.Fatal("ABCDABCDABCD (3 reps) should trigger at threshold=3, but got nil")
	}
	t.Logf("period-4 loop detected as expected: %s", err.Error())
}
