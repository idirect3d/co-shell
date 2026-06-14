// Author: L.Shuang
// Created: 2026-06-14
// Last Modified: 2026-06-14
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
// IMPLIED, BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
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

// TestLoopDetector_RepeatLines verifies that the exact line repetition
// detection catches repeated lines that exceed the threshold.
func TestLoopDetector_RepeatLines(t *testing.T) {
	ld := NewLoopDetector(3, 10)

	lineA := "这是一行测试内容，用于生成重复文本文件。每行固定八十个字循环往复生成多行文本"
	lineB := "这是第二行测试内容，用于生成重复文本文件。每行固定八十个字循环往复生成多行文本"
	lineC := "这是第三行测试内容，用于生成重复文本文件。每行固定八十个字循环往复生成多行文本"

	lines := []string{lineA, lineB, lineC, lineA, lineB, lineC}
	content := strings.Join(lines, "\n")
	err := ld.AddChunk(content, time.Now())
	if err != nil {
		t.Fatalf("should not trigger after 2 repeats of each line, got: %v", err)
	}

	content2 := lineA + "\n" + lineB + "\n"
	err = ld.AddChunk(content2, time.Now())
	if err == nil {
		t.Fatal("should trigger after lineA reaches threshold=3, but got nil")
	}
	ldErr, ok := err.(*LoopDetectedError)
	if !ok {
		t.Fatalf("expected *LoopDetectedError, got %T", err)
	}
	if ldErr.repeatCount < 3 {
		t.Fatalf("expected repeatCount >= 3, got %d", ldErr.repeatCount)
	}
	t.Logf("loop detected as expected: %s", err.Error())
}

// TestLoopDetector_ShortLineFilter verifies that short lines are ignored.
func TestLoopDetector_ShortLineFilter(t *testing.T) {
	ld := NewLoopDetector(3, 50)

	shortLine := "short"
	content := strings.Repeat(shortLine+"\n", 10)
	err := ld.AddChunk(content, time.Now())
	if err != nil {
		t.Fatalf("short lines should be ignored, got: %v", err)
	}
}

// TestLoopDetector_CrossChunk verifies cross-chunk line handling.
func TestLoopDetector_CrossChunk(t *testing.T) {
	ld := NewLoopDetector(3, 10)

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

	assertChunk(lineA+"\n"+lineA+"\n", false)
	assertChunk(lineA+"\n", true)
	t.Log("cross-chunk test passed")
}

// TestLoopDetector_UserReportedCase tests the exact scenario reported by the user:
// three different Chinese regulation lines, each repeated 9+ times.
func TestLoopDetector_UserReportedCase(t *testing.T) {
	ld := NewLoopDetector(5, 50)

	lineA := "金融监管部门持续加强信息科技风险管理，要求金融机构建立健全信息安全管理体系，完善技术应急预案，确保业务连续性。"
	lineB := "网络安全防护方面，监管机构要求落实等级保护制度，加强关键信息基础设施安全保护，定期开展安全风险评估。"
	lineC := "数据安全管理方面，金融机构需严格执行数据分类分级制度，加强个人信息保护，防范数据泄露和滥用风险。"

	var allLines []string
	for i := 0; i < 9; i++ {
		allLines = append(allLines, lineA, lineB, lineC)
	}

	chunk := strings.Join(allLines, "\n")
	err := ld.AddChunk(chunk, time.Now())
	if err == nil {
		t.Fatal("should trigger: each line appears 9 times >= threshold 5, but got nil")
	}
	ldErr, ok := err.(*LoopDetectedError)
	if !ok {
		t.Fatalf("expected *LoopDetectedError, got %T", err)
	}
	t.Logf("loop detected as expected: line repeated %d times (threshold=%d)", ldErr.repeatCount, ldErr.threshold)
}

// TestLoopDetector_StreamedChars simulates line-by-line token-level streaming.
func TestLoopDetector_StreamedChars(t *testing.T) {
	ld := NewLoopDetector(3, 50)
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

// TestLoopDetector_NewlineSeparateChunk tests the critical scenario where
// the "\n" character arrives as a separate chunk (independent SSE event).
// This is how Qwen models behave: content then "\n" as separate tokens.
func TestLoopDetector_NewlineSeparateChunk(t *testing.T) {
	ld := NewLoopDetector(5, 50)

	lineA := "金融监管部门持续加强信息科技风险管理，推动银行业金融机构建立健全信息系统治理架构和网络安全防护体系，确保关键信息基础设施安全稳定运行。"
	lineB := "网络安全法及配套法规体系不断完善，金融行业需严格落实等级保护制度，加强关键信息基础设施保护，建立网络安全监测预警和信息通报机制。"
	lineC := "数据安全管理要求日益严格，金融机构应建立健全数据分类分级保护制度，加强个人信息保护，完善数据全生命周期安全管理机制和应急响应体系。"

	// Simulate Qwen-style streaming: content in small chunks, "\n" as separate chunk
	// 10 cycles of A,B,C each with "\n" after
	for cycle := 0; cycle < 8; cycle++ {
		// Simulate: content token by token, then "\n" as separate event
		// In reality these would be small tokens, but for the test we send full line + "\n"
		for _, line := range []string{lineA, lineB, lineC} {
			// Send content (no trailing \n)
			err := ld.AddChunk(line, time.Now())
			if err != nil {
				t.Fatalf("unexpected trigger after content at cycle %d: %v", cycle, err)
			}
			// Send \n as SEPARATE chunk (exactly what Qwen does)
			err = ld.AddChunk("\n", time.Now())
			if err != nil {
				t.Logf("loop triggered after %d cycles: %s", cycle+1, err.Error())
				return
			}
		}
	}
	t.Fatal("should trigger after 8 cycles (each line appears 8 times >= threshold 5), but never detected")
}

// TestLoopDetector_NoNewlineAfterLine tests the case where a line is completed
// without trailing \n, and the next chunk starts fresh.
func TestLoopDetector_NoNewlineAfterLine(t *testing.T) {
	ld := NewLoopDetector(3, 50)
	lineA := "金融监管部门持续加强信息科技风险管理，要求金融机构建立健全信息安全管理体系，完善技术应急预案，确保业务连续性。"
	err := ld.AddChunk(lineA+"\n", time.Now())
	if err != nil {
		t.Fatalf("should not trigger at count=1, got: %v", err)
	}
	if ld.accumulated.Len() > 0 {
		t.Fatalf("expected empty accumulated after line with \\n, got %q", ld.accumulated.String())
	}
}

// TestLoopDetector_MultiBlockChunk tests when multiple complete blocks arrive
// in a single chunk.
func TestLoopDetector_MultiBlockChunk(t *testing.T) {
	ld := NewLoopDetector(3, 50)
	lineA := "金融监管部门持续加强信息科技风险管理，要求金融机构建立健全信息安全管理体系，完善技术应急预案，确保业务连续性。"
	lineB := "网络安全防护方面，监管机构要求落实等级保护制度，加强关键信息基础设施安全保护，定期开展安全风险评估。"

	chunk := lineA + "\n" + lineA + "\n" + lineB + "\n" + lineB + "\n"
	err := ld.AddChunk(chunk, time.Now())
	if err != nil {
		t.Fatalf("should not trigger at count=2 for each line, got: %v", err)
	}
	err = ld.AddChunk(lineA+"\n", time.Now())
	if err == nil {
		t.Fatal("should trigger after lineA reaches count=3, got nil")
	}
	t.Logf("loop detected: %s", err.Error())
}
