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
	"sync"
	"syscall"

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
	ruleHandler     *cmd.RuleHandler
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
	inputMode  string // "enhanced" or "stdio"

	// FEATURE-201: ESC key monitoring during LLM output
	escWg  sync.WaitGroup // tracks ESC monitor goroutine
	userIO agent.UserIO   // current UserIO for interaction
}

func New(cfg *config.Config, s *store.DualStore, mcpMgr *mcp.Manager, ag *agent.Agent) *REPL {
	r := &REPL{
		cfg:             cfg,
		store:           s,
		mcpMgr:          mcpMgr,
		agent:           ag,
		settingsHandler: cmd.NewSettingsHandler(cfg, ag, s),
		mcpHandler:      cmd.NewMCPHandler(cfg, mcpMgr),
		ruleHandler:     cmd.NewRuleHandler(cfg),
		memoryHandler:   cmd.NewMemoryHandler(s),
		contextHandler:  cmd.NewContextHandler(s),
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
	r.configHandler.SetHandlers(r.mcpHandler, r.ruleHandler, r.memoryHandler,
		r.contextHandler, r.listHandler, r.imageHandler, r.planHandler,
		r.sessionHandler, r.modelHandler, r.sectionHandler, r.modeHandler,
		r.settingsHandler)
	return r
}

func (r *REPL) SetVersion(ver, bld string) { r.version = ver; r.build = bld }
func (r *REPL) SetInputMode(mode string)   { r.inputMode = mode }

func (r *REPL) readLine(prompt string) (string, error) {
	switch r.inputMode {
	case "enhanced":
		ei := NewEnhancedInput(prompt, r.history)
		input, err := ei.ReadLine()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(input), nil
	case "stdio":
		fmt.Print(prompt)
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			return "", scanner.Err()
		}
		return strings.TrimSpace(scanner.Text()), nil
	default:
		ei := NewEnhancedInput(prompt, r.history)
		input, err := ei.ReadLine()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(input), nil
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
		fmt.Printf("%s 检测到 PostgreSQL 连接，开始自动同步本地数据到远端...\n", ep.Info)
		if err := r.store.PG().MigrateFromBolt(r.store.Bolt); err != nil {
			log.Warn("Auto-migration failed (non-fatal): %v", err)
			fmt.Printf("%s  数据同步部分失败: %v\n", ep.Warning, err)
		} else {
			fmt.Printf("%s 数据同步完成!\n", ep.Success)
		}
	} else {
		r.store.SetAutoSync(false)
		fmt.Printf("%s PostgreSQL 已连接（自动同步已关闭）\n", ep.Info)
	}
}

func (r *REPL) Run() error {
	r.printWelcome()
	r.syncDB()
	r.loadHistory()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})

	go func() {
		select {
		case <-sigCh:
			fmt.Println("\n" + i18n.T(i18n.KeyGoodbye))
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

		input, err := r.readLine(prompt)
		if err != nil {
			if err.Error() == "interrupt" {
				fmt.Println("\n" + i18n.T(i18n.KeyGoodbye))
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
			fmt.Printf("\n%s 提示: 你输入的内容以 '.' 开头，这可能是想执行内置命令。\n", ep.Warning)
			fmt.Printf("  内置命令应用 ':' 开头，例如 :settings、:model 等。\n")
			fmt.Printf("  你想把这个内容发送给 LLM 处理吗？\n\n")
			fmt.Printf("  请选择: [Enter] 发送给 LLM  [C] 取消: ")
			// Use readLine which handles both enhanced and stdio modes
			response, _ := r.readLine("")
			response = strings.TrimSpace(strings.ToLower(response))
			if response == "c" {
				fmt.Printf("%s 已取消\n", ep.Warning)
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
	fmt.Println(i18n.T(i18n.KeyGoodbye))
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

	var result string
	var err error

	switch command {
	case ":settings", ":set":
		result, err = r.settingsHandler.Handle(args)
	case ":mcp":
		result, err = r.mcpHandler.Handle(args)
	case ":rule":
		result, err = r.ruleHandler.Handle(args)
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
	case ":body-add":
		result, err = r.handleBodyAdd(args)
	case ":body-remove":
		result, err = r.handleBodyRemove(args)
	case ":body-display":
		result, err = r.handleBodyDisplay(args)
	case ":new":
		r.agent.Reset()
		fmt.Printf("%s%s\n", ep.Success, i18n.T(i18n.KeyHelpNew))
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
		r.handleAgentInput("")
		return
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
		fmt.Printf("%s 已弹出最后一条消息，内容如下：\n%s\n", ep.Info, poppedContent)
		fmt.Println("请编辑后按 Enter 提交，或直接按 Enter 跳过：")
		edited, err := r.readLine("✏️ ")
		if err != nil {
			return
		}
		edited = strings.TrimSpace(edited)
		if edited == "" {
			fmt.Println("已跳过，消息已移除。")
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
		fmt.Printf("%s%s: %v\n", ep.Error, i18n.T(i18n.KeyError), err)
		return
	}
	if num < 1 || num > len(entries) {
		fmt.Println(i18n.TF(i18n.KeyListInvalid, len(entries)))
		return
	}
	input := entries[num-1].Input
	fmt.Printf("%s%s\n", ep.Info, input)
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
		return "", fmt.Errorf("用法: .body-add key=value")
	}
	if r.cfg.LLM.BodyAdditions == nil {
		r.cfg.LLM.BodyAdditions = make(map[string]string)
	}
	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("无效格式 %q，请使用 key=value 格式", arg)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			return "", fmt.Errorf("属性名不能为空")
		}
		r.cfg.LLM.BodyAdditions[key] = value
	}
	r.agent.GetLLMClient().SetBodyAdditions(r.cfg.LLM.BodyAdditions)
	if err := r.cfg.Save(); err != nil {
		return "", fmt.Errorf("保存配置失败: %w", err)
	}
	return fmt.Sprintf("已添加 %d 个自定义属性到 LLM 请求体", len(args)), nil
}

func (r *REPL) handleBodyRemove(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("用法: .body-remove key")
	}
	if r.cfg.LLM.BodyAdditions == nil {
		return "", fmt.Errorf("没有自定义属性可删除")
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
		return "", fmt.Errorf("未找到指定的属性")
	}
	r.agent.GetLLMClient().SetBodyAdditions(r.cfg.LLM.BodyAdditions)
	if err := r.cfg.Save(); err != nil {
		return "", fmt.Errorf("保存配置失败: %w", err)
	}
	return fmt.Sprintf("已删除 %d 个自定义属性", removed), nil
}

func (r *REPL) handleBodyDisplay(args []string) (string, error) {
	if len(r.cfg.LLM.BodyAdditions) == 0 {
		return "没有自定义属性", nil
	}
	var sb strings.Builder
	sb.WriteString("LLM 请求体自定义属性:\n")
	for key, value := range r.cfg.LLM.BodyAdditions {
		sb.WriteString(fmt.Sprintf("  %s = %s\n", key, value))
	}
	return sb.String(), nil
}

func (r *REPL) handleSystemCommand(command string) {
	ep := config.GetEmojiPrefixes(r.cfg.LLM.EmojiEnabled)
	if r.cfg.LLM.ShowCommand {
		fmt.Printf("%s%s\n", ep.CommandInput, command)
	}
	if r.cfg.LLM.ShellSessionEnabled {
		output, err := r.agent.ExecuteViaShellSessionWithOutput(command)
		if err != nil {
			if output != "" {
				fmt.Print(output)
			}
			fmt.Printf("%s%s: %v\n", ep.Error, i18n.T(i18n.KeyCmdFailed), err)
			return
		}
		if output != "" {
			fmt.Printf("%s%s\n", ep.OutputTitle, output)
		}
		return
	}
	output, err := r.agent.ExecuteCommandDirectly(command)
	if err != nil {
		if output != "" {
			fmt.Print(output)
		}
		fmt.Printf("%s%s: %v\n", ep.Error, i18n.T(i18n.KeyCmdFailed), err)
		return
	}
	if output != "" {
		fmt.Printf("%s%s\n", ep.OutputTitle, output)
	}
}

// handleAgentInput sends natural language input to the agent.
// In enhanced input mode, sets up ESC monitoring via a goroutine that polls stdin.
func (r *REPL) handleAgentInput(input string) {
	ctx := context.Background()
	ep := config.GetEmojiPrefixes(r.cfg.LLM.EmojiEnabled)

	fmt.Println()
	fmt.Printf("%s%s\n", ep.LlmOutput, r.agent.Said())

	// FEATURE-201: In enhanced mode, create EnhancedIO, set it on the agent,
	// and start ESC monitor goroutine.
	var stopMonitor func()
	if r.inputMode == "enhanced" {
		log.Debug("REPL.handleAgentInput: setting up EnhancedIO and ESC monitor")
		eio := NewEnhancedIO(r.history)
		// Start raw mode
		if err := eio.startRaw(); err != nil {
			log.Warn("REPL.handleAgentInput: cannot set raw mode: %v", err)
		} else {
			r.agent.SetIO(eio)
			r.userIO = eio
			stopMonitor = r.startESCMonitor()
		}
	}

	_, err := r.agent.RunStream(ctx, input, r.streamCallback)

	// Stop ESC monitor and clean up raw mode
	if stopMonitor != nil {
		log.Debug("REPL.handleAgentInput: stopping ESC monitor")
		stopMonitor()
	}
	if r.userIO != nil {
		if eio, ok := r.userIO.(*EnhancedIO); ok {
			eio.stopRaw()
		}
		r.userIO = nil
		// Reset agent's UserIO so agent defaults back to fmtIO for any remaining output
		r.agent.SetIO(nil)
	}

	if err != nil {
		fmt.Printf("%s%s: %v\n", ep.Error, i18n.T(i18n.KeyProcessFailed), err)
		fmt.Println(i18n.T(i18n.KeyCheckConfig))
	}
}

// streamCallback handles streaming events from the agent.
// In enhanced mode (userIO != nil), delegates output to userIO.Print which
// automatically handles \r\n conversion. In stdio mode, uses direct fmt.Print.
func (r *REPL) streamCallback(eventType string, content string) {
	ep := config.GetEmojiPrefixes(r.cfg.LLM.EmojiEnabled)

	// out prints via UserIO when available (handles \r\n conversion),
	// else falls back to direct fmt.Print.
	out := func(args ...interface{}) {
		if r.userIO != nil {
			r.userIO.Print(args...)
		} else {
			fmt.Print(args...)
		}
	}
	outF := func(format string, args ...interface{}) {
		if r.userIO != nil {
			r.userIO.Printf(format, args...)
		} else {
			fmt.Printf(format, args...)
		}
	}

	switch eventType {
	case "content_chunk":
		out(content)
	case "thinking_chunk":
		out(content)
	case "content":
		out(ep.LlmOutput)
		out(content)
		out("\n")
	case "thinking":
		out(ep.Thinking)
		out(content)
		out("\n")
	case "command":
		out("\n")
		out(ep.CommandInput)
		out(content)
		out("\n")
	case "output":
		out("\n")
		out(ep.OutputTitle)
		out("\n")
		out(ep.OutputSep)
		out("\n")
		out(content)
		out("\n")
		out(ep.OutputSep)
		out("\n")
	case "tool_call":
		out("\n")
		out(ep.ToolCallInput)
		out(content)
		out("\n")
	case "token_iter":
		outF("\n────────────────────────────────────────────────────────────────────────────────\n")
		var prompt, completion, total, maxLen int
		var ft, inTPS, outTPS string
		if _, err := fmt.Sscanf(content, "prompt=%d completion=%d total=%d max=%d ft=%s in_tps=%s out_tps=%s",
			&prompt, &completion, &total, &maxLen, &ft, &inTPS, &outTPS); err == nil {
			pct := 0.0
			if maxLen > 0 && total > 0 {
				pct = float64(total) * 100.0 / float64(maxLen)
			}
			if maxLen == 0 {
				out(" (模型最大长度未知) ")
			}
			out(fmt.Sprintf(i18n.T(i18n.KeyTokenUsageDisplay), ft, prompt, inTPS, completion, outTPS, total, pct))
			out("\n")
		}
		outF("────────────────────────────────────────────────────────────────────────────────\n")
	case "token_task":
		outF("\n────────────────────────────────────────────────────────────────────────────────\n")
		var prompt, completion, total int
		if _, err := fmt.Sscanf(content, "prompt=%d completion=%d total=%d", &prompt, &completion, &total); err == nil {
			out(fmt.Sprintf("本次任务 Token 总计: 输入=%d, 输出=%d, 总计=%d\n", prompt, completion, total))
		}
		outF("────────────────────────────────────────────────────────────────────────────────\n")
	case "info":
		out(content)
	case "warning":
		out(ep.Warning)
		out(content)
		out("\n")
	case "error":
		out(ep.Error)
		out(content)
		out("\n")
	case "done":
		out("\n")
	}
}

func (r *REPL) printWelcome() {
	visionIndicator := ""
	if r.cfg.LLM.VisionSupport {
		visionIndicator = " 👀"
	}
	fmt.Printf("co-shell v%s [BUILD-%s]%s\n", r.version, r.build, visionIndicator)
	fmt.Println("Copyright (c) 2026 L.Shuang - Type ':help' for usage.")
	if r.cfg.LLM.ShowLogo {
		fmt.Println(logoData)
	}
}

func (r *REPL) printHelp() {
	fmt.Println(i18n.T(i18n.KeyHelpTitle))
	fmt.Println()
	fmt.Println(i18n.T(i18n.KeyHelpNLTitle))
	fmt.Println(i18n.T(i18n.KeyHelpNLDesc))
	fmt.Println()
	fmt.Println(i18n.T(i18n.KeyHelpBuiltinTitle))
	fmt.Println(i18n.T(i18n.KeyHelpConfig))
	fmt.Println(i18n.T(i18n.KeyHelpSettings))
	fmt.Println(i18n.T(i18n.KeyHelpMCP))
	fmt.Println(i18n.T(i18n.KeyHelpRule))
	fmt.Println(i18n.T(i18n.KeyHelpMemory))
	fmt.Println(i18n.T(i18n.KeyHelpContext))
	fmt.Println(i18n.T(i18n.KeyHelpHistory))
	fmt.Println(i18n.T(i18n.KeyHelpSession))
	fmt.Println(i18n.T(i18n.KeyHelpImage))
	fmt.Println(i18n.T(i18n.KeyHelpPlan))
	fmt.Println(i18n.T(i18n.KeyHelpBodyAdd))
	fmt.Println(i18n.T(i18n.KeyHelpBodyRemove))
	fmt.Println(i18n.T(i18n.KeyHelpBodyDisplay))
	fmt.Println(i18n.T(i18n.KeyHelpNew))
	fmt.Println(i18n.T(i18n.KeyHelpModel))
	fmt.Println(i18n.T(i18n.KeyHelpSection))
	fmt.Println(i18n.T(i18n.KeyHelpMode))
	fmt.Println(i18n.T(i18n.KeyHelpContinue))
	fmt.Println(i18n.T(i18n.KeyHelpSimulate))
	fmt.Println(i18n.T(i18n.KeyHelpHelp))
	fmt.Println(i18n.T(i18n.KeyHelpExit))
	fmt.Println()
	fmt.Println(i18n.T(i18n.KeyHelpExampleTitle))
	prefix := i18n.T(i18n.KeyEmojiPrefixUser)
	fmt.Println("    " + prefix + i18n.T(i18n.KeyHelpExample1))
	fmt.Println("    " + prefix + i18n.T(i18n.KeyHelpExample2))
	fmt.Println("    " + prefix + i18n.T(i18n.KeyHelpExample3))
	fmt.Println("    " + prefix + i18n.T(i18n.KeyHelpExample4))
	fmt.Println("    " + prefix + i18n.T(i18n.KeyHelpExample5))
}

func (r *REPL) cleanup() {
	fmt.Print(i18n.T(i18n.KeyCleaningUp))
	// Persist non-system messages before closing resources
	if err := r.agent.PersistSessionNonSystem(); err != nil {
		log.Warn("Failed to persist non-system session on REPL exit: %v", err)
	}
	if err := r.mcpMgr.Close(); err != nil {
		fmt.Printf(" MCP error: %v", err)
	}
	if err := r.store.Close(); err != nil {
		fmt.Printf(" DB error: %v", err)
	}
	fmt.Println(i18n.T(i18n.KeyDone))
}
