package repl

import (
	"bufio"
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/cmd"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/mcp"
	"github.com/idirect3d/co-shell/store"
)

//go:embed logo.md
var logoData string

var commandPattern = regexp.MustCompile(commandPatternString())

func commandPatternString() string {
	if runtime.GOOS == "windows" {
		return `^[a-zA-Z0-9._~\\:/-]+(\s+.*)?$`
	}
	return `^[a-zA-Z0-9._/~-]+(\s+.*)?$`
}

var windowsBuiltins = map[string]bool{
	"dir": true, "copy": true, "del": true, "erase": true, "move": true,
	"ren": true, "rename": true, "type": true, "cd": true, "chdir": true,
	"md": true, "mkdir": true, "rd": true, "rmdir": true, "cls": true,
	"echo": true, "set": true, "path": true, "prompt": true, "title": true,
	"date": true, "time": true, "ver": true, "vol": true, "label": true,
	"pushd": true, "popd": true, "where": true, "find": true, "findstr": true,
	"more": true, "sort": true, "pause": true, "color": true, "help": true,
	"break": true, "call": true, "exit": true, "for": true, "goto": true,
	"if": true, "rem": true, "shift": true, "start": true,
	"assoc": true, "ftype": true, "dpath": true, "subst": true,
}

func IsDirectCommand(input string) (string, bool) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", false
	}
	if !commandPattern.MatchString(trimmed) {
		return "", false
	}
	firstWord := strings.Fields(trimmed)[0]
	_, err := exec.LookPath(firstWord)
	if err == nil {
		return trimmed, true
	}
	if runtime.GOOS == "windows" && windowsBuiltins[strings.ToLower(firstWord)] {
		return trimmed, true
	}
	return "", false
}

type BuiltinHandler interface {
	Handle(args []string) (string, error)
}

type REPL struct {
	cfg             *config.Config
	store           *store.DualStore
	mcpMgr          *mcp.Manager
	agent           *agent.Agent
	settingsHandler *cmd.SettingsHandler
	mcpHandler      *cmd.MCPHandler
	memoryHandler   *cmd.MemoryHandler
	contextHandler  *cmd.ContextHandler
	listHandler     *cmd.ListHandler
	imageHandler    *cmd.ImageHandler
	planHandler     *cmd.PlanHandler
	sessionHandler  *cmd.SessionHandler
	modelHandler    *cmd.ModelHandler
	sectionHandler  *cmd.SectionHandler
	modeHandler     *cmd.ModeHandler
	configHandler   *cmd.ConfigHandler
	simulateHandler *cmd.SimulateHandler

	history    []string
	historyPos int
	version    string
	build      string
	inputMode  string // "tui" or "stdio"

	// outputFormat is "text" (default) or "json" (FEATURE-307b, CLI-only).
	// json forces the stdio session and suppresses decorative output.
	outputFormat string

	// FEATURE-307b: the session owns the input source (StdioSource /
	// unified InputReader) and the per-run agent I/O assembly. Created in
	// Run() via sessionFactories[inputMode].
	session SessionIO

	// renderer is the per-run event renderer installed by
	// session.Acquire during handleAgentInput; nil between runs.
	renderer agent.EventRenderer
}

func New(cfg *config.Config, s *store.DualStore, mcpMgr *mcp.Manager, ag *agent.Agent) *REPL {
	r := &REPL{
		cfg:             cfg,
		store:           s,
		mcpMgr:          mcpMgr,
		agent:           ag,
		settingsHandler: cmd.NewSettingsHandler(cfg, ag, s),
		mcpHandler:      cmd.NewMCPHandler(cfg, mcpMgr),
		memoryHandler:   cmd.NewMemoryHandler(s),
		contextHandler:  cmd.NewContextHandler(ag, s),
		listHandler:     cmd.NewListHandler(s),
		imageHandler:    cmd.NewImageHandler(ag),
		planHandler:     cmd.NewPlanHandler(ag.TaskPlanManager()),
		sessionHandler:  cmd.NewSessionHandler(ag, cfg),
		modelHandler:    cmd.NewModelHandler(cfg, ag),
		sectionHandler:  cmd.NewSectionHandler(cfg),
		modeHandler:     cmd.NewModeHandler(cfg, ag),
		configHandler:   cmd.NewConfigHandler(cfg, ag),
		simulateHandler: cmd.NewSimulateHandler(ag, cfg),
	}
	r.configHandler.SetScanner(bufio.NewScanner(os.Stdin))
	r.configHandler.SetHandlers(r.mcpHandler, r.memoryHandler,
		r.contextHandler, r.listHandler, r.imageHandler, r.planHandler,
		r.sessionHandler, r.modelHandler, r.sectionHandler, r.modeHandler,
		r.settingsHandler)
	return r
}

func (r *REPL) SetVersion(ver, bld string)      { r.version = ver; r.build = bld }
func (r *REPL) SetInputMode(mode string)        { r.inputMode = mode }
func (r *REPL) SetOutputFormat(format string)   { r.outputFormat = format }

// rawActive reports whether the terminal is currently held in raw mode by a
// tui session (FEATURE-307b: the reader now lives inside the session).
func (r *REPL) rawActive() bool {
	if ts, ok := r.session.(*tuiSession); ok && ts.reader != nil {
		return ts.reader.RawActive()
	}
	return false
}

// decorationsOn reports whether decorative (non-event) output such as the
// welcome banner, the input prompt and the Said line is shown. Suppressed in
// JSON output mode so stdout carries only JSON event lines. The outputFormat
// check comes first because cleanup() clears the session.
func (r *REPL) decorationsOn() bool {
	if r.outputFormat == "json" {
		return false
	}
	return r.session == nil || r.session.Interactive()
}

// pauseReader pauses the tui session's unified input reader (no-op for
// stdio sessions). Used while a builtin command wizard owns stdin.
func (r *REPL) pauseReader() {
	if ts, ok := r.session.(*tuiSession); ok && ts.reader != nil {
		ts.reader.Pause()
	}
}

// resumeReader resumes the tui session's unified input reader (no-op for
// stdio sessions).
func (r *REPL) resumeReader() {
	if ts, ok := r.session.(*tuiSession); ok && ts.reader != nil {
		ts.reader.Resume()
	}
}

// rawPrintf prints user-visible output, applying \r\n conversion while the
// unified input reader holds the terminal in raw mode (tui). In cooked mode
// (stdio / reader paused) it behaves exactly like fmt.Printf.
func (r *REPL) rawPrintf(format string, args ...interface{}) {
	s := fmt.Sprintf(format, args...)
	if r.rawActive() {
		s = strings.ReplaceAll(s, "\n", "\r\n")
	}
	fmt.Print(s)
}

// rawPrint prints user-visible output with raw-mode \r\n conversion.
func (r *REPL) rawPrint(args ...interface{}) {
	s := fmt.Sprint(args...)
	if r.rawActive() {
		s = strings.ReplaceAll(s, "\n", "\r\n")
	}
	fmt.Print(s)
}

// rawPrintln prints a line, returning the cursor to column 0 first and using
// \r\n while the terminal is in raw mode.
func (r *REPL) rawPrintln(args ...interface{}) {
	s := fmt.Sprint(args...)
	if r.rawActive() {
		s = strings.ReplaceAll(s, "\n", "\r\n")
		fmt.Print("\r" + s + "\r\n")
	} else {
		fmt.Println(s)
	}
}

func (r *REPL) syncDB() {
	if r.store.PG() == nil {
		r.store.SetAutoSync(false)
		return
	}
	ep := config.GetEmojiPrefixes(r.cfg.LLM.EmojiEnabled)
	if r.cfg.DB.AutoSync {
		r.store.SetAutoSync(true)
		if r.decorationsOn() {
			fmt.Print(i18n.TF(i18n.KeyDBSyncStart, ep.Info))
		}
		if err := r.store.PG().MigrateFromBolt(r.store.Bolt); err != nil {
			log.Warn("Auto-migration failed (non-fatal): %v", err)
			if r.decorationsOn() {
				fmt.Print(i18n.TF(i18n.KeyDBSyncPartial, ep.Warning, err))
			}
		} else if r.decorationsOn() {
			fmt.Print(i18n.TF(i18n.KeyDBSyncComplete, ep.Success))
		}
	} else {
		r.store.SetAutoSync(false)
		if r.decorationsOn() {
			fmt.Print(i18n.TF(i18n.KeyDBConnectedNoSync, ep.Info))
		}
	}
}

func (r *REPL) Run() error {
	// FEATURE-307b: in JSON output mode all decorations (welcome banner,
	// prompt, Said line, DB sync notices) are suppressed so stdout carries
	// only JSON event lines.
	if r.outputFormat != "json" {
		r.printWelcome()
	}
	r.syncDB()
	r.loadHistory()

	// FEATURE-307b: create the session for the configured input mode. The
	// tui session owns the unified input reader (single stdin goroutine +
	// raw terminal mode); the stdio session reads lines synchronously
	// through a persistent StdioSource (no reader goroutine needed) so piped
	// input survives across iterations. A tui start failure falls back to
	// stdio, preserving the historical behavior.
	deps := SessionDeps{
		Cfg:          r.cfg,
		HistoryFn:    func() []string { return r.history },
		OutputFormat: r.outputFormat,
		Ag:           r.agent,
	}
	factory := sessionFactories[r.inputMode]
	if factory == nil {
		factory = sessionFactories["tui"]
	}
	session, err := factory(deps)
	if err != nil {
		log.Warn("REPL: raw mode unavailable (%v), falling back to stdio", err)
		r.inputMode = "stdio"
		if session, err = sessionFactories["stdio"](deps); err != nil {
			return err
		}
	}
	r.session = session

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})

	go func() {
		select {
		case <-sigCh:
			if r.decorationsOn() {
				r.rawPrintln("\n" + i18n.T(i18n.KeyGoodbye))
			}
			r.cleanup()
			os.Exit(0)
		case <-done:
			return
		}
	}()

	for {
		ep := config.GetEmojiPrefixes(r.cfg.LLM.EmojiEnabled)
		prompt := ep.UserInput
		if r.cfg.LLM.VisionSupport {
			prompt = ep.VisionUserInput
		}
		// Insert mode name into prompt before "]>"
		modeName := r.cfg.LLM.WorkMode
		if modeName == "" || modeName == "default" {
			modeName = "act"
		}
		prompt = strings.Replace(prompt, "]> ", "]["+modeName+"]> ", 1)

		input, err := r.session.ReadLine(prompt)
		if err != nil {
			if err.Error() == "interrupt" {
				if r.decorationsOn() {
					r.rawPrintln("\n" + i18n.T(i18n.KeyGoodbye))
				}
				r.cleanup()
				os.Exit(0)
			}
			break
		}
		if input == "" {
			continue
		}

		r.saveHistory(input)
		if input == "exit" || input == "quit" || input == ":exit" {
			break
		}
		if input == "help" || input == ":help" || input == "?" {
			r.printHelp()
			continue
		}
		// FEATURE-273: If input starts with ".", check if the first word is
		// an executable in the current directory. If so, execute it directly.
		// If not, warn the user that ":" should be used for builtin commands
		// and ask if they want to send it to LLM anyway.
		if strings.HasPrefix(input, ".") {
			firstWord := strings.Fields(input)[0]
			isLocalExec := false
			if info, err := os.Stat(firstWord); err == nil && !info.IsDir() && info.Mode().Perm()&0111 != 0 {
				isLocalExec = true
			}
			if isLocalExec {
				r.handleSystemCommand(input)
				continue
			}
			// Not a local executable: warn user about ":" prefix, then ask
			ep := config.GetEmojiPrefixes(r.cfg.LLM.EmojiEnabled)
			r.rawPrint(i18n.TF(i18n.KeyDotPrefixHint1, ep.Warning))
			r.rawPrint(i18n.T(i18n.KeyDotPrefixHint2))
			r.rawPrint(i18n.T(i18n.KeyDotPrefixAskLLM))
			r.rawPrint(i18n.T(i18n.KeyDotPrefixChoose))
			// Read the user's choice through the session (handles both tui
			// and stdio sessions).
			response, _ := r.session.ReadLine("")
			response = strings.TrimSpace(strings.ToLower(response))
			if response == "c" {
				r.rawPrint(i18n.TF(i18n.KeyDotPrefixCancelled, ep.Warning))
				continue
			}
			// Fall through to handleAgentInput
		}

		if strings.HasPrefix(input, ":") {
			r.handleBuiltin(input)
			continue
		}
		if num, err := strconv.Atoi(input); err == nil && num > 0 {
			r.handleHistoryReExecute(num)
			continue
		}
		if cmd, ok := IsDirectCommand(input); ok {
			r.handleSystemCommand(cmd)
			continue
		}
		r.handleAgentInput(input)
	}

	close(done)
	r.cleanup()
	if r.decorationsOn() {
		r.rawPrintln(i18n.T(i18n.KeyGoodbye))
	}
	return nil
}

func (r *REPL) loadHistory() {
	entries, err := r.store.LoadHistory()
	if err != nil {
		log.Warn("Cannot load history: %v", err)
		r.history = []string{}
		return
	}
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	r.history = entries
	r.historyPos = len(r.history)
}

func (r *REPL) saveHistory(input string) {
	if err := r.store.SaveHistory(input); err != nil {
		log.Warn("Cannot save history: %v", err)
	}
	// Update in-memory history so current session entries appear in Up/Down navigation.
	r.history = append(r.history, input)
	r.historyPos = len(r.history)
}

func (r *REPL) handleBuiltin(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}
	ep := config.GetEmojiPrefixes(r.cfg.LLM.EmojiEnabled)
	command := parts[0]
	args := parts[1:]

	// P2.5: while a builtin command wizard owns stdin (it reads directly via
	// DefaultUserIO in cooked mode), pause the unified input reader so its
	// goroutine does not compete for input bytes and the terminal returns to
	// cooked mode (proper \r\n line discipline + visible echo).
	resumed := false
	r.pauseReader()
	defer func() {
		if !resumed {
			r.resumeReader()
		}
	}()

	var result string
	var err error

	switch command {
	case ":settings", ":set":
		result, err = r.settingsHandler.Handle(args)
	case ":mcp":
		result, err = r.mcpHandler.Handle(args)
	case ":memory":
		result, err = r.memoryHandler.Handle(args)
	case ":context":
		result, err = r.contextHandler.Handle(args)
	case ":history":
		result, err = r.listHandler.HandleHistory(args)
	case ":session":
		result, err = r.sessionHandler.Handle(args)
	case ":image":
		result, err = r.imageHandler.Handle(args)
	case ":plan":
		result, err = r.planHandler.Handle(args)
	case ":vault":
		if r.agent != nil && r.agent.VaultStore() != nil {
			result, err = r.handleVaultCommand(args)
		} else {
			err = fmt.Errorf("vault not available - vault store not initialized")
		}
	case ":body-add":
		result, err = r.handleBodyAdd(args)
	case ":body-remove":
		result, err = r.handleBodyRemove(args)
	case ":body-display":
		result, err = r.handleBodyDisplay(args)
	case ":new":
		// Flush current session messages to DB, then create a new empty session
		if err := r.agent.FlushCurrentSession(); err != nil {
			log.Warn("Failed to flush current session: %v", err)
		}
		// Find the next "New session N" number
		nextN := 1
		sessionNumRe := regexp.MustCompile(`(\d+)$`)
		if entries, err := r.agent.Store().ListNamedSessions(); err == nil {
			maxN := 0
			for _, e := range entries {
				if m := sessionNumRe.FindStringSubmatch(e.Title); m != nil {
					if suffix, err := strconv.Atoi(m[1]); err == nil && suffix > maxN {
						maxN = suffix
					}
				}
			}
			if maxN > 0 {
				nextN = maxN + 1
			}
		}
		r.agent.Reset()
		// Create session entry directly (force save even with empty messages)
		now := time.Now()
		randBytes := make([]byte, 4)
		randBytes[0] = byte(now.Nanosecond() & 0xFF)
		randBytes[1] = byte(now.Nanosecond() >> 8 & 0xFF)
		randBytes[2] = byte(now.Second() & 0xFF)
		randBytes[3] = byte(now.Minute() & 0xFF)
		sessionID := fmt.Sprintf("sess-%s-%08x", now.Format("20060102150405"), randBytes)
		title := fmt.Sprintf(i18n.T(i18n.KeyNewSessionTitle), nextN)
		entry := &store.SessionEntry{
			ID:           sessionID,
			Title:        title,
			Keywords:     "",
			SystemPrompt: "",
			Messages:     []byte("[]"), // empty messages array
			MessageCount: 0,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := r.agent.Store().SaveNamedSession(entry); err != nil {
			log.Warn("Failed to save new session: %v", err)
		}
		r.agent.SetCurrentSessionID(sessionID)
		if err := r.agent.Store().SaveCurrentSessionID(sessionID); err != nil {
			log.Warn("Failed to save current session ID: %v", err)
		}
		fmt.Print(i18n.TF(i18n.KeyNewSessionCreated, ep.Success, title))
		return
	case ":model":
		result, err = r.modelHandler.Handle(args)
	case ":section":
		result, err = r.sectionHandler.Handle(args)
	case ":mode":
		result, err = r.modeHandler.Handle(args)
	case ":config":
		result, err = r.configHandler.Handle(args)
	case ":db":
		result, err = r.settingsHandler.HandleDB(args)
	case ":simulate":
		result, err = r.simulateHandler.Handle(args)
	case ":continue":
		// Resume the reader before running the agent so ESC/Ctrl+C monitoring
		// and EnhancedIO event consumption work normally.
		resumed = true
		r.resumeReader()
		r.handleAgentInput("")
		return
	case ":reset":
		rh := cmd.NewResetHandler(r.agent)
		result, err = rh.Handle(args)
	default:
		fmt.Printf("%s%s\n", ep.Error, i18n.T(i18n.KeyUnknownCommand))
		return
	}

	if err != nil {
		fmt.Printf("%s%s: %v\n", ep.Error, i18n.T(i18n.KeyError), err)
		return
	}
	// Handle special POP: result from :session pop — allow user to edit and resubmit
	if strings.HasPrefix(result, "POP:") {
		poppedContent := result[4:]
		fmt.Print(i18n.TF(i18n.KeySessionPopEdit, ep.Info, poppedContent))
		fmt.Println(i18n.T(i18n.KeySessionPopEditHint))
		edited, err := r.session.ReadLine("✏️ ")
		if err != nil {
			return
		}
		edited = strings.TrimSpace(edited)
		if edited == "" {
			fmt.Println(i18n.T(i18n.KeySessionPopSkipped))
			return
		}
		// Resubmit with modified content
		if strings.HasPrefix(edited, ":") {
			r.handleBuiltin(edited)
			return
		}
		if cmd, ok := IsDirectCommand(edited); ok {
			r.handleSystemCommand(cmd)
			return
		}
		r.handleAgentInput(edited)
		return
	}
	fmt.Println(result)
	if command == ":settings" || command == ":set" {
		r.agent.SetShowLlmThinking(r.cfg.LLM.ShowLlmThinking)
		r.agent.SetShowLlmContent(r.cfg.LLM.ShowLlmContent)
		r.agent.SetShowTool(r.cfg.LLM.ShowTool)
		r.agent.SetShowToolInput(r.cfg.LLM.ShowToolInput)
		r.agent.SetShowToolOutput(r.cfg.LLM.ShowToolOutput)
		r.agent.SetShowCommand(r.cfg.LLM.ShowCommand)
		r.agent.SetShowCommandOutput(r.cfg.LLM.ShowCommandOutput)
		r.agent.SetToolCallEnabled(r.cfg.LLM.ToolCallEnabled)
	}
}

func (r *REPL) handleHistoryReExecute(num int) {
	ep := config.GetEmojiPrefixes(r.cfg.LLM.EmojiEnabled)
	entries, err := r.store.ListHistory()
	if err != nil {
		r.rawPrintf("%s%s: %v\n", ep.Error, i18n.T(i18n.KeyError), err)
		return
	}
	if num < 1 || num > len(entries) {
		r.rawPrintln(i18n.TF(i18n.KeyListInvalid, len(entries)))
		return
	}
	input := entries[num-1].Input
	r.rawPrintf("%s%s\n", ep.Info, input)
	if strings.HasPrefix(input, ":") {
		r.handleBuiltin(input)
		return
	}
	if cmd, ok := IsDirectCommand(input); ok {
		r.handleSystemCommand(cmd)
		return
	}
	r.handleAgentInput(input)
}

func (r *REPL) handleBodyAdd(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("%s", i18n.T(i18n.KeyBodyAddUsage))
	}
	if r.cfg.LLM.BodyAdditions == nil {
		r.cfg.LLM.BodyAdditions = make(map[string]string)
	}
	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("%s", i18n.TF(i18n.KeyBodyAddInvalidFmt, arg))
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			return "", fmt.Errorf("%s", i18n.T(i18n.KeyBodyAddEmptyKey))
		}
		r.cfg.LLM.BodyAdditions[key] = value
	}
	r.agent.GetLLMClient().SetBodyAdditions(r.cfg.LLM.BodyAdditions)
	if err := r.cfg.Save(); err != nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyBodyAddSaveFail), err)
	}
	return fmt.Sprintf(i18n.T(i18n.KeyBodyAddDone), len(args)), nil
}

func (r *REPL) handleBodyRemove(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("%s", i18n.T(i18n.KeyBodyRemoveUsage))
	}
	if r.cfg.LLM.BodyAdditions == nil {
		return "", fmt.Errorf("%s", i18n.T(i18n.KeyBodyRemoveNone))
	}
	removed := 0
	for _, key := range args {
		key = strings.TrimSpace(key)
		if _, exists := r.cfg.LLM.BodyAdditions[key]; exists {
			delete(r.cfg.LLM.BodyAdditions, key)
			removed++
		}
	}
	if removed == 0 {
		return "", fmt.Errorf("%s", i18n.T(i18n.KeyBodyRemoveNotFound))
	}
	r.agent.GetLLMClient().SetBodyAdditions(r.cfg.LLM.BodyAdditions)
	if err := r.cfg.Save(); err != nil {
		return "", fmt.Errorf(i18n.T(i18n.KeyBodyAddSaveFail), err)
	}
	return fmt.Sprintf(i18n.T(i18n.KeyBodyRemoveDone), removed), nil
}

func (r *REPL) handleBodyDisplay(args []string) (string, error) {
	if len(r.cfg.LLM.BodyAdditions) == 0 {
		return i18n.T(i18n.KeyBodyDisplayEmpty), nil
	}
	var sb strings.Builder
	sb.WriteString(i18n.T(i18n.KeyBodyDisplayTitle))
	for key, value := range r.cfg.LLM.BodyAdditions {
		sb.WriteString(fmt.Sprintf("  %s = %s\n", key, value))
	}
	return sb.String(), nil
}

func (r *REPL) handleSystemCommand(command string) {
	ep := config.GetEmojiPrefixes(r.cfg.LLM.EmojiEnabled)
	if r.cfg.LLM.ShowCommand {
		r.rawPrintf("%s%s\n", ep.CommandInput, command)
	}
	if r.cfg.LLM.ShellSessionEnabled {
		output, err := r.agent.ExecuteViaShellSessionWithOutput(command)
		if err != nil {
			if output != "" {
				r.rawPrint(output)
			}
			r.rawPrintf("%s%s: %v\n", ep.Error, i18n.T(i18n.KeyCmdFailed), err)
			return
		}
		if output != "" {
			r.rawPrintf("%s%s\n", ep.OutputTitle, output)
		}
		return
	}
	output, err := r.agent.ExecuteCommandDirectly(command)
	if err != nil {
		if output != "" {
			r.rawPrint(output)
		}
		r.rawPrintf("%s%s: %v\n", ep.Error, i18n.T(i18n.KeyCmdFailed), err)
		return
	}
	if output != "" {
		r.rawPrintf("%s%s\n", ep.OutputTitle, output)
	}
}

// handleAgentInput sends natural language input to the agent.
// The per-run I/O assembly (UserIO installation, ESC consumer, command
// hooks, event renderer) is delegated to the session (FEATURE-307b).
func (r *REPL) handleAgentInput(input string) {
	ctx := context.Background()
	ep := config.GetEmojiPrefixes(r.cfg.LLM.EmojiEnabled)

	// The Said line is decoration, not an event: suppressed in JSON output
	// mode so stdout carries only JSON event lines.
	if r.decorationsOn() {
		r.rawPrintln()
		r.rawPrintf("%s%s\n", ep.LlmOutput, r.agent.Said())
	}

	// FEATURE-307b: the session installs the UserIO the agent uses during
	// RunStream (tui: EnhancedIO + ESC consumer + command hooks; stdio:
	// StdioIO) and returns the event renderer for this run. Without a
	// UserIO, the agent falls back to fmtIO which has a no-op ReadKey
	// returning (0, nil) immediately — causing infinite loops on
	// confirmation prompts.
	renderer, release := r.session.Acquire(r.agent)
	r.renderer = renderer

	_, err := r.agent.RunStream(ctx, input, r.streamCallback)

	r.renderer = nil
	release()

	if err != nil {
		if !r.decorationsOn() {
			// JSON output mode: keep stdout a pure JSON event stream; report
			// failures on stderr.
			fmt.Fprintf(os.Stderr, "%s: %v\n%s\n", i18n.T(i18n.KeyProcessFailed), err, i18n.T(i18n.KeyCheckConfig))
			return
		}
		r.rawPrintf("%s%s: %v\n", ep.Error, i18n.T(i18n.KeyProcessFailed), err)
		r.rawPrintln(i18n.T(i18n.KeyCheckConfig))
	}
}

// streamCallback handles streaming events from the agent by delegating to
// the per-run renderer installed by session.Acquire (FEATURE-307b). The fmt
// fallback only guards defensive paths (cleanup or direct callback reuse).
func (r *REPL) streamCallback(ev agent.StreamEvent) {
	renderer := r.renderer
	if renderer == nil {
		ep := config.GetEmojiPrefixes(r.cfg.LLM.EmojiEnabled)
		renderer = agent.NewLineRenderer(agent.NewDefaultUserIO(), ep, agent.StreamModeREPL)
	}
	renderer.Render(ev)
}

func (r *REPL) printWelcome() {
	visionIndicator := ""
	if r.cfg.LLM.VisionSupport {
		visionIndicator = " 👀"
	}
	r.rawPrintf("co-shell v%s [BUILD-%s]%s\n", r.version, r.build, visionIndicator)
	r.rawPrintln("Copyright (c) 2026 L.Shuang - Type ':help' for usage.")
	if r.cfg.LLM.ShowLogo {
		r.rawPrintln(logoData)
	}
}

func (r *REPL) printHelp() {
	r.rawPrintln(i18n.T(i18n.KeyHelpTitle))
	r.rawPrintln()
	r.rawPrintln(i18n.T(i18n.KeyHelpNLTitle))
	r.rawPrintln(i18n.T(i18n.KeyHelpNLDesc))
	r.rawPrintln()
	r.rawPrintln(i18n.T(i18n.KeyHelpBuiltinTitle))
	r.rawPrintln(i18n.T(i18n.KeyHelpConfig))
	r.rawPrintln(i18n.T(i18n.KeyHelpSettings))
	r.rawPrintln(i18n.T(i18n.KeyHelpMCP))
	r.rawPrintln(i18n.T(i18n.KeyHelpMemory))
	r.rawPrintln(i18n.T(i18n.KeyHelpContext))
	r.rawPrintln(i18n.T(i18n.KeyHelpHistory))
	r.rawPrintln(i18n.T(i18n.KeyHelpSession))
	r.rawPrintln(i18n.T(i18n.KeyHelpImage))
	r.rawPrintln(i18n.T(i18n.KeyHelpPlan))
	r.rawPrintln(i18n.T(i18n.KeyHelpVault))
	r.rawPrintln(i18n.T(i18n.KeyHelpBodyAdd))
	r.rawPrintln(i18n.T(i18n.KeyHelpBodyRemove))
	r.rawPrintln(i18n.T(i18n.KeyHelpBodyDisplay))
	r.rawPrintln(i18n.T(i18n.KeyHelpNew))
	r.rawPrintln(i18n.T(i18n.KeyHelpModel))
	r.rawPrintln(i18n.T(i18n.KeyHelpSection))
	r.rawPrintln(i18n.T(i18n.KeyHelpMode))
	r.rawPrintln(i18n.T(i18n.KeyHelpContinue))
	r.rawPrintln(i18n.T(i18n.KeyHelpSimulate))
	r.rawPrintln(i18n.T(i18n.KeyHelpHelp))
	r.rawPrintln(i18n.T(i18n.KeyHelpExit))
	r.rawPrintln()
	r.rawPrintln(i18n.T(i18n.KeyHelpExampleTitle))
	prefix := i18n.T(i18n.KeyEmojiPrefixUser)
	r.rawPrintln("    " + prefix + i18n.T(i18n.KeyHelpExample1))
	r.rawPrintln("    " + prefix + i18n.T(i18n.KeyHelpExample2))
	r.rawPrintln("    " + prefix + i18n.T(i18n.KeyHelpExample3))
	r.rawPrintln("    " + prefix + i18n.T(i18n.KeyHelpExample4))
}

func (r *REPL) cleanup() {
	decorations := r.decorationsOn()
	if decorations {
		r.rawPrint(i18n.T(i18n.KeyCleaningUp))
	}
	// FEATURE-307b: the session owns the input source; closing it stops the
	// unified input reader and restores the terminal (tui).
	if r.session != nil {
		_ = r.session.Close()
		r.session = nil
	}
	// Persist non-system messages before closing resources
	if err := r.agent.PersistSessionNonSystem(); err != nil {
		log.Warn("Failed to persist non-system session on REPL exit: %v", err)
	}
	if err := r.mcpMgr.Close(); err != nil {
		r.rawPrintf(" MCP error: %v", err)
	}
	if err := r.store.Close(); err != nil {
		r.rawPrintf(" DB error: %v", err)
	}
	if decorations {
		r.rawPrintln(i18n.T(i18n.KeyDone))
	}
}
