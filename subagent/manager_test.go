// Package subagent - managed sub-agent lifecycle tests (FEATURE-563):
// batch launch, per-id workspaces, background timeout, wait, kill with
// process-group escalation, run reuse, persistence and shutdown cleanup.
//
// The tests drive the real Manager against a fake co-shell script: the script
// emits the JSON-Lines stream the manager parses, so no LLM or real engine is
// involved.
//
// Author: L.Shuang
// Created: 2026-09-27
// MIT License - Copyright (c) 2026 L.Shuang

package subagent

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// fakeSubAgentScript is the executable fixture standing in for a co-shell
// child. FAKE_SLEEP keeps it running, FAKE_EXIT sets its exit code and
// FAKE_IGNORE_TERM makes it survive SIGTERM so the SIGKILL escalation can be
// observed.
const fakeSubAgentScript = `#!/bin/sh
echo "{\"type\":\"thinking\",\"text\":\"env=$CO_SHELL_SUB_AGENT/$CO_SHELL_SUB_AGENT_ID\"}"
echo '{"type":"thinking_chunk","text":"weighing options"}'
echo '{"type":"content_chunk","text":"intermediate answer"}'
echo '{"type":"tool_call","meta":{"name":"read_file"},"text":"read_file"}'
echo '{"type":"content_chunk","text":"final answer"}'
if [ "$FAKE_IGNORE_TERM" = "1" ]; then
  trap '' TERM
  i=0
  while [ $i -lt 100 ]; do sleep 0.2; i=$((i+1)); done
  exit 0
fi
if [ -n "$FAKE_SLEEP" ]; then sleep "$FAKE_SLEEP"; fi
exit ${FAKE_EXIT:-0}
`

// fakeCoShell writes the fixture script into a temp dir and returns its path.
func fakeCoShell(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake co-shell fixture requires a POSIX shell")
	}
	path := filepath.Join(t.TempDir(), "fake-co-shell")
	if err := os.WriteFile(path, []byte(fakeSubAgentScript), 0o755); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// fakePersister is the in-memory Persister standing in for the store.
type fakePersister struct {
	mu   sync.Mutex
	data map[string]string
}

func newFakePersister() *fakePersister {
	return &fakePersister{data: make(map[string]string)}
}

func (f *fakePersister) SaveMemory(key, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[key] = value
	return nil
}

func (f *fakePersister) GetMemory(key string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.data[key]
	return v, ok, nil
}

// testSessionID is the session the fixture managers work in: the sub-agent
// pool, the process table and the workspaces are all per session (FEATURE-565).
const testSessionID = "sess-test"

// newTestManager builds a manager over a fresh workspace with the fake
// co-shell as the child executable.
func newTestManager(t *testing.T, persister Persister) (*Manager, string) {
	t.Helper()
	ws := t.TempDir()
	m := NewManager(Options{
		Workspace:   ws,
		ExecPath:    fakeCoShell(t),
		Persister:   persister,
		NoStoreLoad: true,
	})
	m.SetSessionID(testSessionID)
	t.Cleanup(m.Close)
	return m, ws
}

// agentDir is the workspace of one sub-agent of the fixture session.
func agentDir(ws, id string) string {
	return filepath.Join(ws, "sub-agents", testSessionID, id)
}

// waitGone polls until the process disappears or the deadline expires. A
// killed child stays visible to kill(pid,0) until its reaper calls Wait and
// collects the zombie, so the check must be asynchronous.
func waitGone(t *testing.T, pid int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("process %d is still alive after %s", pid, timeout)
}

// TestStartBatchCreatesIsolatedWorkspaces covers the batch launch contract:
// one call starts several sub-agents, each in
// {workspace}/sub-agents/{session}/{id}/, and each returns its own result
// (UC-0001: 批量创建 + 结果提取, FEATURE-565: 每会话目录).
func TestStartBatchCreatesIsolatedWorkspaces(t *testing.T) {
	m, ws := newTestManager(t, nil)
	procs, err := m.Start(context.Background(), []Task{
		{ID: "A1", Instruction: "first"},
		{ID: "A2", Instruction: "second"},
	}, 10*time.Second)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(procs) != 2 {
		t.Fatalf("Start returned %d procs, want 2", len(procs))
	}
	for _, p := range procs {
		if p.Status != StatusCompleted {
			t.Errorf("%s status = %s, want completed", p.ID, p.Status)
		}
		if p.WaitTimedOut {
			t.Errorf("%s: WaitTimedOut must be false for a finished process", p.ID)
		}
		if p.Result != "final answer" {
			t.Errorf("%s result = %q, want %q", p.ID, p.Result, "final answer")
		}
		if len(p.ToolCalls) != 1 || p.ToolCalls[0] != "read_file" {
			t.Errorf("%s tool calls = %v, want [read_file]", p.ID, p.ToolCalls)
		}
		want := agentDir(ws, p.ID)
		if p.Workspace != want {
			t.Errorf("%s workspace = %q, want %q", p.ID, p.Workspace, want)
		}
		if want2 := filepath.Join(want, "run-1.jsonl"); p.LogPath != want2 {
			t.Errorf("%s log = %q, want %q", p.ID, p.LogPath, want2)
		}
		if _, err := os.Stat(p.LogPath); err != nil {
			t.Errorf("%s log file missing: %v", p.ID, err)
		}
	}

	// The child env carries the sub-agent markers, which the fixture echoes.
	logData, err := os.ReadFile(procs[0].LogPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(logData), "env=true/A1") {
		t.Errorf("child env markers missing from %s", procs[0].LogPath)
	}
}

// TestStartValidation covers the batch guards: no task, pool id syntax/range,
// duplicate slots and an empty instruction (UC-0001: id 校验, FEATURE-565:
// id 池与防路径穿越).
func TestStartValidation(t *testing.T) {
	m, _ := newTestManager(t, nil)
	ctx := context.Background()

	if _, err := m.Start(ctx, nil, time.Second); err == nil {
		t.Error("Start(nil tasks) must fail")
	}
	// The id must name a slot: everything that is not A<digits> is a format
	// error, which also rules out path traversal ids.
	for _, bad := range []string{"../evil", "a1", "A", "B1", "scheduled-3"} {
		_, err := m.Start(ctx, []Task{{ID: bad, Instruction: "x"}}, time.Second)
		pe, ok := AsPoolError(err)
		if !ok {
			t.Fatalf("Start(id=%q) error = %v, want a PoolError", bad, err)
		}
		if pe.Kind != PoolErrIDFormat {
			t.Errorf("Start(id=%q) kind = %v, want PoolErrIDFormat", bad, pe.Kind)
		}
	}
	// A blank id is "required", not a format error (UC: id 必填).
	if _, err := m.Start(ctx, []Task{{ID: "  ", Instruction: "x"}}, time.Second); err != nil {
		pe, ok := AsPoolError(err)
		if !ok || pe.Kind != PoolErrIDRequired {
			t.Errorf("Start(blank id) = %v, want a PoolErrIDRequired", err)
		}
	}
	// A0 and anything past the pool are range errors, not format errors.
	for _, bad := range []string{"A0", "A6"} {
		_, err := m.Start(ctx, []Task{{ID: bad, Instruction: "x"}}, time.Second)
		pe, ok := AsPoolError(err)
		if !ok || pe.Kind != PoolErrIDOutOfRange {
			t.Errorf("Start(id=%q) = %v, want a PoolErrIDOutOfRange", bad, err)
		}
		if ok && pe.Max != DefaultMaxSubAgents {
			t.Errorf("Start(id=%q) max = %d, want %d", bad, pe.Max, DefaultMaxSubAgents)
		}
	}
	// The same slot twice in one batch.
	_, err := m.Start(ctx, []Task{{ID: "A1", Instruction: "x"}, {ID: "A1", Instruction: "y"}}, time.Second)
	if pe, ok := AsPoolError(err); !ok || pe.Kind != PoolErrIDDuplicate {
		t.Errorf("Start(duplicate A1) = %v, want a PoolErrIDDuplicate", err)
	} else if pe.Max != DefaultMaxSubAgents {
		t.Errorf("duplicate error max = %d, want %d", pe.Max, DefaultMaxSubAgents)
	}
	if _, err := m.Start(ctx, []Task{{ID: "A1", Instruction: "   "}}, time.Second); err == nil {
		t.Error("Start with a blank instruction must fail")
	}
	if procs := m.List(nil); len(procs) != 0 {
		t.Errorf("rejected batches must not register processes, got %d", len(procs))
	}
}

// TestTimeoutLeavesProcessRunning covers the timeout-to-background rule: the
// launch window expiring never kills the child, and a later Wait observes its
// real completion (UC-0001: 超时转后台不杀 + wait).
func TestTimeoutLeavesProcessRunning(t *testing.T) {
	t.Setenv("FAKE_SLEEP", "1")
	m, _ := newTestManager(t, nil)

	procs, err := m.Start(context.Background(), []Task{{ID: "A1", Instruction: "think"}}, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(procs) != 1 {
		t.Fatalf("Start returned %d procs, want 1", len(procs))
	}
	if p := procs[0]; !p.WaitTimedOut || p.Status != StatusRunning {
		t.Fatalf("after timeout: status=%s timedOut=%v, want running/true", p.Status, p.WaitTimedOut)
	}
	if !processAlive(procs[0].PID) {
		t.Fatalf("process %d must keep running after the wait window expired", procs[0].PID)
	}

	final := m.Wait([]string{"A1"}, 15*time.Second)
	if len(final) != 1 {
		t.Fatalf("Wait returned %d procs, want 1", len(final))
	}
	if final[0].Status != StatusCompleted {
		t.Errorf("status after Wait = %s, want completed", final[0].Status)
	}
	if final[0].Result != "final answer" {
		t.Errorf("result after Wait = %q, want %q", final[0].Result, "final answer")
	}
	if final[0].WaitTimedOut {
		t.Error("WaitTimedOut must be cleared once the process finished")
	}
}

// TestWaitZeroTimeoutReturnsImmediately confirms Wait only observes state.
func TestWaitZeroTimeoutReturnsImmediately(t *testing.T) {
	t.Setenv("FAKE_SLEEP", "30")
	m, _ := newTestManager(t, nil)

	if _, err := m.Start(context.Background(), []Task{{ID: "A1", Instruction: "x"}}, 100*time.Millisecond); err != nil {
		t.Fatalf("Start: %v", err)
	}
	start := time.Now()
	procs := m.Wait(nil, 0)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("Wait(timeout=0) blocked for %s, want immediate", elapsed)
	}
	if len(procs) != 1 || procs[0].Status != StatusRunning {
		t.Fatalf("Wait = %+v, want the single running process", procs)
	}
	if !processAlive(procs[0].PID) {
		t.Error("Wait must never kill a process")
	}
}

// TestKillTerminatesOwnProcessGroup covers the kill contract: the child runs
// in its own process group and Kill leaves nothing alive (UC-0001: kill).
func TestKillTerminatesOwnProcessGroup(t *testing.T) {
	t.Setenv("FAKE_SLEEP", "30")
	m, _ := newTestManager(t, nil)

	procs, err := m.Start(context.Background(), []Task{{ID: "A1", Instruction: "loop"}}, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	pid := procs[0].PID
	if procs[0].PGID != pid {
		t.Errorf("pgid = %d, want its own group %d", procs[0].PGID, pid)
	}

	killed, err := m.Kill([]string{"A1"})
	if err != nil {
		t.Fatalf("Kill: %v", err)
	}
	if len(killed) != 1 {
		t.Fatalf("Kill returned %d procs, want 1", len(killed))
	}
	if killed[0].Status != StatusKilled {
		t.Errorf("status = %s, want killed", killed[0].Status)
	}
	if killed[0].ExitCode != -1 {
		t.Errorf("exit code = %d, want -1 for a killed process", killed[0].ExitCode)
	}
	waitGone(t, pid, 3*time.Second)
}

// TestKillEscalatesToSigkill covers the SIGTERM -> SIGKILL escalation: a child
// that traps SIGTERM must still be gone when Kill returns.
func TestKillEscalatesToSigkill(t *testing.T) {
	t.Setenv("FAKE_IGNORE_TERM", "1")
	m, _ := newTestManager(t, nil)

	procs, err := m.Start(context.Background(), []Task{{ID: "A1", Instruction: "ignore"}}, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	pid := procs[0].PID
	if !processAlive(pid) {
		t.Fatalf("fixture %d exited too early to test the escalation", pid)
	}

	// Make sure the fixture really ignores SIGTERM first: without that the test
	// would pass even if Kill never escalated to SIGKILL.
	if err := killGroup(pid, procs[0].PGID, syscall.SIGTERM); err != nil {
		t.Fatalf("signal the fixture: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if !processAlive(pid) {
		t.Skip("fixture died on SIGTERM; the SIGKILL escalation cannot be observed")
	}

	if _, err := m.Kill([]string{"A1"}); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	waitGone(t, pid, 3*time.Second)
}

// TestKillValidation covers the kill guards.
func TestKillValidation(t *testing.T) {
	m, _ := newTestManager(t, nil)
	if _, err := m.Kill(nil); err == nil {
		t.Error("Kill(nil) must fail")
	}
	if _, err := m.Kill([]string{"A5"}); err == nil {
		t.Error("Kill with an idle pool id must fail")
	}
}

// TestReuseIncrementsRun covers slot reuse: a finished slot restarts in the
// same workspace with the next run number while the earlier log is kept, and a
// running slot is refused (UC-0001: 复用, FEATURE-565: 非运行中 id 可复用).
func TestReuseIncrementsRun(t *testing.T) {
	m, ws := newTestManager(t, nil)
	ctx := context.Background()

	first, err := m.Start(ctx, []Task{{ID: "A1", Instruction: "one"}}, 10*time.Second)
	if err != nil {
		t.Fatalf("Start #1: %v", err)
	}
	if first[0].Run != 1 {
		t.Fatalf("first run = %d, want 1", first[0].Run)
	}

	second, err := m.Start(ctx, []Task{{ID: "A1", Instruction: "two"}}, 10*time.Second)
	if err != nil {
		t.Fatalf("Start #2: %v", err)
	}
	if second[0].Run != 2 {
		t.Errorf("second run = %d, want 2", second[0].Run)
	}
	if second[0].Workspace != agentDir(ws, "A1") {
		t.Errorf("reused workspace = %q, want %q", second[0].Workspace, agentDir(ws, "A1"))
	}
	for _, name := range []string{"run-1.jsonl", "run-2.jsonl"} {
		if _, err := os.Stat(filepath.Join(agentDir(ws, "A1"), name)); err != nil {
			t.Errorf("%s missing after reuse: %v", name, err)
		}
	}
}

// TestRunningIDIsRefused covers the "slot already running" guard.
func TestRunningIDIsRefused(t *testing.T) {
	t.Setenv("FAKE_SLEEP", "30")
	m, _ := newTestManager(t, nil)

	if _, err := m.Start(context.Background(), []Task{{ID: "A1", Instruction: "x"}}, 100*time.Millisecond); err != nil {
		t.Fatalf("Start: %v", err)
	}
	_, err := m.Start(context.Background(), []Task{{ID: "A1", Instruction: "y"}}, time.Second)
	pe, ok := AsPoolError(err)
	if !ok || pe.Kind != PoolErrIDBusy {
		t.Fatalf("second Start on a running slot = %v, want a PoolErrIDBusy", err)
	}
	if pe.Run != 1 || pe.PID <= 0 {
		t.Errorf("busy error = run %d pid %d, want run 1 and a live pid", pe.Run, pe.PID)
	}
}

// TestFailedExitIsRecorded checks a non-zero exit becomes failed.
func TestFailedExitIsRecorded(t *testing.T) {
	t.Setenv("FAKE_EXIT", "3")
	m, _ := newTestManager(t, nil)

	procs, err := m.Start(context.Background(), []Task{{ID: "A1", Instruction: "fail"}}, 10*time.Second)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if procs[0].Status != StatusFailed {
		t.Errorf("status = %s, want failed", procs[0].Status)
	}
	if procs[0].ExitCode != 3 {
		t.Errorf("exit code = %d, want 3", procs[0].ExitCode)
	}
}

// TestStatusBlockAndOutputs covers the injected <sub_agents> block and the
// output listing used by sub_agent_status (UC-0001: 状态注入/产出清单,
// FEATURE-565: 只列运行中 + max).
func TestStatusBlockAndOutputs(t *testing.T) {
	t.Setenv("FAKE_SLEEP", "30")
	m, _ := newTestManager(t, nil)
	if _, err := m.Start(context.Background(), []Task{{ID: "A1", Instruction: "x"}}, 100*time.Millisecond); err != nil {
		t.Fatalf("Start: %v", err)
	}

	block := m.StatusBlock()
	for _, want := range []string{
		`<sub_agents max="5">`, `</sub_agents>`, `id="A1"`, `status="running"`, `run="1"`,
	} {
		if !strings.Contains(block, want) {
			t.Errorf("StatusBlock missing %q:\n%s", want, block)
		}
	}

	outDir := filepath.Join(m.SubAgentDir(testSessionID, "A1"), "output")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir output: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "note.md"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("write output: %v", err)
	}
	files := m.OutputFiles("A1")
	if len(files) != 1 || files[0] != "note.md" {
		t.Errorf("OutputFiles = %v, want [note.md]", files)
	}
	// The stream is written up front, so the tail is readable while running;
	// the child needs a moment to start and echo its lines.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if tail := m.LogTail("A1", 4096); strings.Contains(tail, "final answer") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("LogTail = %q, want the emitted stream", m.LogTail("A1", 4096))
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := m.Kill([]string{"A1"}); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	// A finished sub-agent leaves the block: only running rows are listed.
	after := m.StatusBlock()
	if strings.Contains(after, `id="A1"`) {
		t.Errorf("StatusBlock still lists a killed sub-agent:\n%s", after)
	}
	if !strings.Contains(after, `max="5"`) {
		t.Errorf("StatusBlock must keep the pool size even when idle:\n%s", after)
	}
}

// TestCloseKillsEveryRunning covers shutdown cleanup: Close kills all running
// sub-agents (process groups included) and is safe to call twice (UC-0003:
// 退出清理).
func TestCloseKillsEveryRunning(t *testing.T) {
	t.Setenv("FAKE_SLEEP", "30")
	m, _ := newTestManager(t, nil)

	procs, err := m.Start(context.Background(), []Task{
		{ID: "A1", Instruction: "x"},
		{ID: "A2", Instruction: "y"},
	}, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	pids := make([]int, 0, len(procs))
	for _, p := range procs {
		pids = append(pids, p.PID)
	}

	m.Close()
	for _, pid := range pids {
		waitGone(t, pid, 3*time.Second)
	}
	for _, p := range m.List(nil) {
		if p.Status != StatusKilled {
			t.Errorf("%s status = %s, want killed", p.ID, p.Status)
		}
	}
	m.Close() // must be idempotent
}

// TestPersistenceRecoversProcessTable covers store persistence: a new manager
// over the same persister recovers the previous process table (UC-0003:
// 持久化恢复).
func TestPersistenceRecoversProcessTable(t *testing.T) {
	persister := newFakePersister()
	ws := t.TempDir()
	exe := fakeCoShell(t)

	m1 := NewManager(Options{Workspace: ws, ExecPath: exe, Persister: persister, NoStoreLoad: true})
	m1.SetSessionID(testSessionID)
	if _, err := m1.Start(context.Background(), []Task{{ID: "A1", Instruction: "x"}}, 10*time.Second); err != nil {
		t.Fatalf("Start: %v", err)
	}
	m1.Close()

	if _, ok := persister.data[storeKey]; !ok {
		t.Fatalf("process table was not persisted under %q", storeKey)
	}

	m2 := NewManager(Options{Workspace: ws, ExecPath: exe, Persister: persister})
	m2.SetSessionID(testSessionID)
	got, ok := m2.Lookup("A1")
	if !ok {
		t.Fatal("recovered manager lost the persisted record")
	}
	if got.Status != StatusCompleted || got.Run != 1 {
		t.Errorf("recovered record = %s run %d, want completed run 1", got.Status, got.Run)
	}
	if got.Result != "final answer" {
		t.Errorf("recovered result = %q, want %q", got.Result, "final answer")
	}
}

// TestPersistenceMarksDeadRunningRecordStale covers the stale rule: a
// persisted "running" record whose process is gone is marked stale on load
// instead of being trusted (UC-0003: 失效记录标 stale).
func TestPersistenceMarksDeadRunningRecordStale(t *testing.T) {
	persister := newFakePersister()
	ws := t.TempDir()
	exe := fakeCoShell(t)

	m1 := NewManager(Options{Workspace: ws, ExecPath: exe, Persister: persister, NoStoreLoad: true})
	m1.SetSessionID(testSessionID)
	if _, err := m1.Start(context.Background(), []Task{{ID: "A1", Instruction: "x"}}, 10*time.Second); err != nil {
		t.Fatalf("Start: %v", err)
	}
	m1.Close()

	payload, ok, err := persister.GetMemory(storeKey)
	if err != nil || !ok {
		t.Fatalf("GetMemory(%q) = (%v, %v)", storeKey, ok, err)
	}
	// Pretend the parent died while the child was still running.
	patched := strings.Replace(payload, `"status":"completed"`, `"status":"running"`, 1)
	if patched == payload {
		t.Fatalf("persisted payload does not carry a status field: %s", payload)
	}
	broken := &fakePersister{data: map[string]string{storeKey: patched}}

	m2 := NewManager(Options{Workspace: ws, ExecPath: exe, Persister: broken})
	m2.SetSessionID(testSessionID)
	got, ok := m2.Lookup("A1")
	if !ok {
		t.Fatal("recovered manager lost the patched record")
	}
	if got.Status != StatusStale {
		t.Errorf("status = %s, want stale for a dead \"running\" record", got.Status)
	}
}

// TestStartInjectsMachineReadableStreamFlags verifies that every launch carries
// the display flags the parent parser depends on — the child inherits
// config.json, where show_tool/show_llm_content may be off (FEATURE-563) — and
// that an explicit opt-out (SetStreamFlags(false)) drops them.
func TestStartInjectsMachineReadableStreamFlags(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture requires a POSIX shell")
	}
	root := t.TempDir()
	argsPath := filepath.Join(root, "args.txt")
	script := filepath.Join(root, "fake-co-shell")
	body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argsPath + "\nexit 0\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	const streamBlock = "--show-tool\non\n--show-llm-content\non\n"
	for _, tc := range []struct {
		name    string
		enabled bool
		want    bool
	}{
		{name: "default", enabled: true, want: true},
		{name: "explicitly disabled", enabled: false, want: false},
	} {
		m := NewManager(Options{
			Workspace:   filepath.Join(root, "ws-"+strings.ReplaceAll(tc.name, " ", "-")),
			ExecPath:    script,
			NoStoreLoad: true,
		})
		m.SetStreamFlags(tc.enabled)
		if _, err := m.Start(context.Background(), []Task{{ID: "A1", Instruction: "run"}}, 10*time.Second); err != nil {
			t.Fatalf("%s: Start: %v", tc.name, err)
		}
		data, err := os.ReadFile(argsPath)
		if err != nil {
			t.Fatalf("%s: read args: %v", tc.name, err)
		}
		got := string(data)
		if has := strings.Contains(got, streamBlock); has != tc.want {
			t.Errorf("%s: launch args = %q, contains stream flags = %v, want %v",
				tc.name, got, has, tc.want)
		}
		if !strings.Contains(got, "--unattended\non\n") {
			t.Errorf("%s: launch args = %q, want --unattended on", tc.name, got)
		}
		if !strings.Contains(got, "--output-format\njson\n") {
			t.Errorf("%s: launch args = %q, want --output-format json", tc.name, got)
		}
		m.Close()
	}
}
