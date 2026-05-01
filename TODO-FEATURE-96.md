# FEATURE-96 Implementation Plan

## Changes Overview

### 1. Rename `get_history_slice` → `get_memory_slice`
- File: `agent/loop.go` - Change tool name in `buildTools()`, rename callback method
- File: `agent/command_tools.go` - Update tool descriptions if referenced

### 2. Add datetime prefix to user/assistant messages
- File: `agent/loop.go` - In `Run()` and `RunStream()`, when adding user/assistant messages, prepend timestamp
- Format: "2026-05-01 09:51:01: "

### 3. Messages pointer (indicator) for LLM context
- Add `messagePointer` field to Agent struct
- When create_task_plan / insert_task_steps / remove_task_steps is called:
  - Format current checklist as assistant message
  - Append to messages (but NOT to memory)
  - Move pointer to end (after the new message)
- `buildContextMessages()` uses pointer to determine start position
- `.session` display shows `*` next to the pointer message

### 4. Session display: mark pointer message with `*`
- File: `cmd/session.go` - Show `*` next to the message at pointer position

### 5. Insert messages into memory when adding to messages
- File: `agent/loop.go` - When adding user/assistant messages, also call `memoryManager.AddMessage()`

### 6. memory_search content length limit + max results
- Add `MemorySearchMaxContentLen` and `MemorySearchMaxResults` to config
- Add CLI flags: `--memory-search-max-content-len`, `--memory-search-max-results`
- Add `.set` commands: `memory-search-max-content-len`, `memory-search-max-results`
- Modify `memory.Search()` to limit results
- Modify `memory.FormatSearchResults()` to truncate content
- Default: M=32, N=1000
