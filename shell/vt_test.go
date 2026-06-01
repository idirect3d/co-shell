// Author: L.Shuang
// Created: 2026-05-31
// Last Modified: 2026-05-31
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
// furnished to do so, subject to the conditions:
// [The above license notice and permission notice shall be included...]
// SOFTWARE.

package shell

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestVT_BasicRender tests basic VT rendering with simple text.
func TestVT_BasicRender(t *testing.T) {
	vt := NewVirtualTerminal(24, 80)

	// Feed text with CR/LF
	vt.Process([]byte("Line 1\r\nLine 2\r\nLine 3\r\n"))
	render := vt.Render()
	fmt.Printf("Basic render:\n%s\n", render)

	if !strings.Contains(render, "Line 1") {
		t.Error("Expected Line 1 in render")
	}
	if !strings.Contains(render, "Line 2") {
		t.Error("Expected Line 2 in render")
	}
}

// TestVT_OSCSequence tests that a long OSC sequence doesn't break the VT.
func TestVT_OSCSequence(t *testing.T) {
	vt := NewVirtualTerminal(24, 80)

	// Long OSC sequence (simulating iTerm2 set working directory URL)
	osc := []byte{0x1b, ']', '6', ';', '1', ';'}
	for i := 0; i < 200; i++ {
		osc = append(osc, byte('A'+i%26))
	}
	osc = append(osc, 0x07) // BEL terminator

	vt.Process(osc)

	// After OSC, normal text should still work
	vt.Process([]byte("Hello after long OSC\r\n"))
	render := vt.Render()
	fmt.Printf("After OSC:\n%s\n", render)

	if !strings.Contains(render, "Hello") {
		t.Error("Expected 'Hello' in render after long OSC")
	}
}

// TestVT_PythonPrompt tests that Python REPL output renders correctly.
func TestVT_PythonPrompt(t *testing.T) {
	vt := NewVirtualTerminal(24, 80)

	// Simulate PTY character-by-character echo of "python3"
	// Each character is preceded by \r to overwrite the line
	chars := []string{"p", "py", "pyt", "pyth", "pytho", "python", "python3"}
	for _, c := range chars {
		vt.Process([]byte("\r" + c))
	}
	vt.Process([]byte("\r\n")) // CR+LF at end of command line

	// Python banner
	vt.Process([]byte("Python 3.12.2 | packaged by conda-forge | (main, Feb 16 2024, 20:54:21) [Clang 16.0.6 ] on darwin\r\n"))
	vt.Process([]byte("Type \"help\", \"copyright\", \"credits\" or \"license\" for more information.\r\n"))

	// Python prompt (no \r\n after it)
	vt.Process([]byte(">>> "))

	render := vt.Render()
	fmt.Printf("Python render:\n%s\n", render)

	// Should show Python banner and prompt
	if !strings.Contains(render, "Python 3.12") {
		t.Errorf("Expected Python version in render, got:\n%s", render)
	}
	if !strings.Contains(render, ">>>") {
		t.Errorf("Expected >>> prompt in render, got:\n%s", render)
	}

	// Count >>> lines - there should be exactly 1
	lines := strings.Split(render, "\n")
	promptCount := 0
	for _, l := range lines {
		if strings.TrimSpace(l) == ">>>" {
			promptCount++
		}
	}
	if promptCount > 1 {
		t.Errorf("Expected only 1 line with >>>, got %d", promptCount)
	}
}

// TestVT_UnterminatedOSC tests behavior when OSC doesn't terminate.
func TestVT_UnterminatedOSC(t *testing.T) {
	vt := NewVirtualTerminal(24, 80)

	// Very long unterminated OSC (no BEL, no ST)
	vt.Process([]byte{0x1b, ']', '7', ';'}) // OSC start
	for i := 0; i < 5000; i++ {
		vt.Process([]byte{'x'})
	}

	// After the OSC guard triggers, this text should be visible
	vt.Process([]byte("AFTER_OSC\r\n"))
	render := vt.Render()
	fmt.Printf("After unterminated OSC:\n%s\n", render)

	if !strings.Contains(render, "AFTER_OSC") {
		t.Error("Expected AFTER_OSC in render - OSC guard may not be working")
	}
}

// TestVT_iTerm2FullSequence replicates the full scenario from the bug report.
func TestVT_iTerm2FullSequence(t *testing.T) {
	vt := NewVirtualTerminal(24, 80)

	// iTerm2 OSC file:// URL
	oscURL := fmt.Sprintf("\x1b]6;1;file://lsmbas.local/Users/direct3d/github/co-shell/work\x07")
	vt.Process([]byte(oscURL))

	// PTY startup output: each character echoed with \r
	echoSeq := ""
	echoSeq += "\r\n\r$ \rp\r$ \rpy\r$ \rpyt\r$ \rpyth\r$ \rpytho\r$ \rpytho\r$ \rpython\r$ \rpython3\r\n"
	vt.Process([]byte(echoSeq))

	// Python banner + prompt
	vt.Process([]byte("Python 3.12.2 | packaged by conda-forge | (main, Feb 16 2024, 20:54:21) [Clang 16.0.6 ] on darwin\r\n"))
	vt.Process([]byte("Type \"help\", \"copyright\", \"credits\" or \"license\" for more information.\r\n"))
	vt.Process([]byte(">>> "))

	render := vt.Render()
	fmt.Printf("Full sequence render:\n%s\n", render)

	lines := strings.Split(render, "\n")

	// Check for >>> prompt - should only appear once
	promptCount := 0
	for _, l := range lines {
		if strings.TrimSpace(l) == ">>>" {
			promptCount++
		}
	}
	if promptCount > 1 {
		t.Errorf(">>> prompt appears %d times (expected 1)\nFull render:\n%s", promptCount, render)
	}

	// Should not have 24 lines of >>>
	if promptCount >= 24 {
		t.Fatal("ALL lines are >>> - VT is broken")
	}
}

// TestVT_RealSession tests the VT with a real shell session.
// This creates an actual PTY session and checks what the VT renders.
// SKIP this test in normal test runs; run with -run TestVT_RealSession
func TestVT_RealSession(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	sess := &Session{}
	_, err := sess.Start()
	if err != nil {
		t.Fatalf("Failed to start session: %v", err)
	}
	defer sess.Close()

	// Wait and check what's in the VT after startup + reset
	time.Sleep(100 * time.Millisecond)
	content, err := sess.GetWindowContent()
	if err != nil {
		t.Fatalf("GetWindowContent failed: %v", err)
	}
	fmt.Printf("After startup+reset VT content:\n%s\nEND\n", content)

	// Send python3 command
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	output, err := sess.Exec(ctx, "python3\n", 1500)
	if err != nil {
		t.Fatalf("Exec failed: %v (output: %s)", err, output)
	}

	fmt.Printf("After python3, VT render:\n%s\nEND\n", output)
	fmt.Printf("Render length: %d bytes, lines: %d\n", len(output), strings.Count(output, "\n")+1)

	// Check if we got a sensible result
	if strings.Contains(output, ">>>") {
		fmt.Println("✓ Python prompt found in output")
	} else {
		fmt.Println("✗ No Python prompt found - checking content")
	}

	// Check for all lines being >>>
	lines := strings.Split(output, "\n")
	allPrompt := true
	for _, l := range lines {
		if strings.TrimSpace(l) != ">>>" {
			allPrompt = false
			break
		}
	}
	if allPrompt {
		t.Error("ALL lines are >>> - VT rendering is broken!")
	}

	// Now execute Python statements in the REPL
	fmt.Println("\n--- Executing Python statements ---")

	// Execute: x = 10
	ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel2()
	output2, err := sess.Exec(ctx2, "x = 10\n", 500)
	if err != nil {
		t.Errorf("Python x=10 failed: %v (output: %s)", err, output2)
	}
	fmt.Printf("After x=10, VT render:\n%s\nEND\n", output2)

	// Execute: y = 20
	ctx3, cancel3 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel3()
	output3, err := sess.Exec(ctx3, "y = 20\n", 500)
	if err != nil {
		t.Errorf("Python y=20 failed: %v (output: %s)", err, output3)
	}
	fmt.Printf("After y=20, VT render:\n%s\nEND\n", output3)

	// Execute: print(x + y)
	ctx4, cancel4 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel4()
	output4, err := sess.Exec(ctx4, "print(x + y)\n", 500)
	if err != nil {
		t.Errorf("Python print(x+y) failed: %v (output: %s)", err, output4)
	}
	fmt.Printf("After print(x+y), VT render:\n%s\nEND\n", output4)

	if !strings.Contains(output4, "30") {
		t.Errorf("Expected '30' in Python output, got:\n%s", output4)
	}
}
