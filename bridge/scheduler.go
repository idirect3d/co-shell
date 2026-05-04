// Author: L.Shuang
// Created: 2026-05-04
// Last Modified: 2026-05-04
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

package bridge

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Mode represents the execution mode.
type Mode int

const (
	ModeSync    Mode = iota // Sync mode: execute one by one, queue subsequent messages
	ModePool                // Pool mode: queue messages, batch process when current task completes
	ModePreempt             // Preempt mode: interrupt current process, start new one immediately
)

// String returns the string representation of the mode.
func (m Mode) String() string {
	switch m {
	case ModeSync:
		return "sync"
	case ModePool:
		return "pool"
	case ModePreempt:
		return "preempt"
	default:
		return "unknown"
	}
}

// ParseMode parses a mode string into a Mode value.
func ParseMode(s string) (Mode, bool) {
	switch strings.ToLower(s) {
	case "sync":
		return ModeSync, true
	case "pool":
		return ModePool, true
	case "preempt":
		return ModePreempt, true
	default:
		return ModeSync, false
	}
}

// Message represents a user message to be processed.
type Message struct {
	ChatID      string
	UserID      string
	Instruction string
	ReplyFunc   func(string, error) // Callback to send the reply

	// InputRequestFunc is called when co-shell is waiting for user input.
	// It receives the current accumulated output and returns a channel that
	// will receive the user's input. The channel is closed when input is received.
	InputRequestFunc func(currentOutput string) <-chan string
}

// Scheduler manages the execution of co-shell processes based on the configured mode.
type Scheduler struct {
	mode     Mode
	executor *Executor
	mu       sync.Mutex

	// Sync/Pool mode fields
	queue   []Message
	running bool

	// Preempt mode fields
	cancel context.CancelFunc

	// Global context for cancellation (e.g., Ctrl+C)
	globalCtx context.Context
}

// NewScheduler creates a new Scheduler with the given mode and executor.
// The ctx parameter is the global context for cancellation (e.g., Ctrl+C).
func NewScheduler(ctx context.Context, mode Mode, executor *Executor) *Scheduler {
	return &Scheduler{
		mode:      mode,
		executor:  executor,
		globalCtx: ctx,
	}
}

// Submit submits a message for execution according to the current mode.
func (s *Scheduler) Submit(msg Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch s.mode {
	case ModeSync:
		s.submitSync(msg)
	case ModePool:
		s.submitPool(msg)
	case ModePreempt:
		s.submitPreempt(msg)
	}
}

// submitSync queues the message and processes sequentially.
func (s *Scheduler) submitSync(msg Message) {
	s.queue = append(s.queue, msg)
	if !s.running {
		s.running = true
		go s.processNext()
	}
}

// processNext processes the next message in the queue.
func (s *Scheduler) processNext() {
	for {
		s.mu.Lock()
		if len(s.queue) == 0 {
			s.running = false
			s.mu.Unlock()
			return
		}
		msg := s.queue[0]
		s.queue = s.queue[1:]
		s.mu.Unlock()

		// Show processing start
		fmt.Println()
		fmt.Printf("⚙️  co-shell 正在处理 \"%s\"\n", msg.Instruction)
		fmt.Println(strings.Repeat("─", 50))

		var output string
		var err error

		if msg.InputRequestFunc != nil {
			// Use interactive mode when InputRequestFunc is provided
			output, err = s.executor.ExecuteInteractive(s.globalCtx, msg.Instruction, msg.InputRequestFunc)
		} else {
			// Fall back to non-interactive mode
			output, err = s.executor.Execute(msg.Instruction)
		}

		// Show completion
		fmt.Println()
		fmt.Println("✅ 处理已完成")
		fmt.Println(strings.Repeat("─", 50))

		msg.ReplyFunc(output, err)
	}
}

// submitPool queues the message and processes in batch when current task completes.
func (s *Scheduler) submitPool(msg Message) {
	s.queue = append(s.queue, msg)
	if !s.running {
		s.running = true
		go s.processBatch()
	}
}

// processBatch processes all queued messages as a single batch.
func (s *Scheduler) processBatch() {
	for {
		s.mu.Lock()
		if len(s.queue) == 0 {
			s.running = false
			s.mu.Unlock()
			return
		}

		// Take all messages from the queue
		batch := make([]Message, len(s.queue))
		copy(batch, s.queue)
		s.queue = s.queue[:0]
		s.mu.Unlock()

		// Merge instructions
		var instructions []string
		for _, msg := range batch {
			instructions = append(instructions, msg.Instruction)
		}
		mergedInstruction := strings.Join(instructions, "\n")

		// Execute the merged instruction
		output, err := s.executor.Execute(mergedInstruction)

		// Reply to each message with the same output
		for _, msg := range batch {
			msg.ReplyFunc(output, err)
		}
	}
}

// submitPreempt interrupts the current process and starts a new one.
func (s *Scheduler) submitPreempt(msg Message) {
	// Cancel any running process
	if s.cancel != nil {
		s.cancel()
	}

	// Create a new cancellable context derived from global context
	ctx, cancel := context.WithCancel(s.globalCtx)
	s.cancel = cancel

	// Execute in background
	go func() {
		output, err := s.executor.ExecuteWithCancel(ctx, msg.Instruction)
		msg.ReplyFunc(output, err)
	}()
}

// Mode returns the current mode.
func (s *Scheduler) Mode() Mode {
	return s.mode
}

// SetMode changes the execution mode.
// If switching from preempt mode, it cancels any running process.
func (s *Scheduler) SetMode(mode Mode) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.mode == ModePreempt && mode != ModePreempt {
		if s.cancel != nil {
			s.cancel()
			s.cancel = nil
		}
	}

	s.mode = mode
}

// QueueLen returns the current queue length.
func (s *Scheduler) QueueLen() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.queue)
}

// IsRunning returns whether a task is currently being executed.
func (s *Scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running || s.cancel != nil
}

// Status returns a human-readable status string.
func (s *Scheduler) Status() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	var parts []string
	parts = append(parts, fmt.Sprintf("mode=%s", s.mode))
	if s.running {
		parts = append(parts, fmt.Sprintf("running=true queue=%d", len(s.queue)))
	} else {
		parts = append(parts, "running=false")
	}
	return strings.Join(parts, " ")
}
