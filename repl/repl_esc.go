// ESC/Ctrl+C event-stream consumer (replaces the legacy unix.Poll monitor).
//
// Author: L.Shuang
// Created: 2026-08-16
// Last Modified: 2026-08-16
// MIT License - Copyright (c) 2026 L.Shuang

package repl

import (
	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/log"
)

// startEscConsumer subscribes to the unified input event stream and translates
// ESC/Ctrl+C events into agent Interrupt/Cancel signals. It replaces the
// legacy unix.Poll-based monitor (repl_esc_posix.go / repl_esc_windows.go),
// eliminating the stdin read race and enabling the same behaviour on Windows.
//
// Priority rule: an active exclusive consumer (ReadLine/ReadKey) owns the
// event — the monitor yields while IsReading; it also yields while a system
// command owns stdin (IsCommandRunning).
func (r *REPL) startEscConsumer() func() {
	return r.reader.Subscribe(func(ev agent.InputEvent) {
		switch ev.Kind {
		case agent.InputEsc, agent.InputCtrlC:
			if io := r.agent.IO(); io != nil && io.IsReading() {
				return
			}
			if r.agent.IsCommandRunning() {
				return
			}
			if ev.Kind == agent.InputEsc {
				log.Info("ESC detected!")
				r.agent.Interrupt()
			} else {
				log.Info("Ctrl+C detected! Cancelling task immediately.")
				r.agent.Cancel()
			}
		}
	})
}
