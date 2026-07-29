#!/usr/bin/env python3
"""Check all XML tool/param tags in i18n system files for {XML_TAG_PREFIX} prefix.
Reports which tags are missing the prefix, categorized by severity.

Usage: python3 bin/check_xml_prefix.py
"""

import re
import sys

PREFIX_PLACEHOLDER = '{XML_TAG_PREFIX}'

# Complete list of known tool and parameter tags that MUST have the prefix
KNOWN_TAGS = {
    # Core tools
    'execute_command', 'read_file', 'search_files', 'write_to_file', 
    'replace_in_file', 'list_files', 'list_code_definition_names',
    'visual_analysis', 'launch_sub_agent', 'schedule_task',
    'track_task_progress', 'view_task_plan',
    'get_memory_slice', 'memory_search', 'delete_memory',
    'update_settings', 'list_settings',
    'ask_followup_question', 'attempt_completion',
    'evaluate_expression', 'reorganize_context',
    'shell_send', 'shell_get_output', 'shell_window_content', 'shell_reset', 'shell_start', 'shell_stop',
    'browser_navigate', 'browser_screenshot', 'browser_click',
    'browser_type', 'browser_evaluate', 'browser_get_rendered_html',
    'browser_scroll', 'browser_get_interactive_elements',
    'browser_go_back', 'browser_go_forward', 'browser_close',
    'vault_list', 'vault_add', 'vault_remove',
    'word_open', 'word_close', 'word_save', 'word_overview',
    'word_read', 'word_table_read', 'word_continue', 'word_erase',
    'word_inspect_style', 'word_format', 'word_style_clone',
    'excel_open', 'excel_close', 'excel_save', 'excel_overview',
    'excel_read', 'excel_edit', 'excel_copy', 'excel_paste',
    'excel_insert', 'excel_delete', 'excel_sheet', 'excel_format',
    # Parameter tags
    'command', 'intent', 'path', 'content', 'mode',
    'start_line', 'end_line', 'timeout_seconds',
    'recursive', 'file_pattern', 'regex',
    'expression', 'url', 'quality', 'full_page', 'x', 'y',
    'text', 'clear', 'session_id', 'sheet',
    'start_row', 'end_row', 'start_col', 'end_col',
    'format', 'max_cells', 'values', 'start_cell',
    'cut', 'target_cell', 'what', 'position', 'count',
    'action', 'name', 'new_name',
    'font_name', 'font_size', 'font_bold', 'font_italic', 'font_underline',
    'font_color', 'fill_color', 'h_align', 'v_align', 'wrap_text',
    'border_style', 'border_color', 'border_top', 'border_bottom',
    'border_left', 'border_right', 'number_format', 'row_height', 'col_width',
    'title', 'description', 'steps', 'status', 'page',
    'sub_agent_name', 'instruction', 'cron',
    'last_from', 'last_to', 'since', 'keywords',
    'settings', 'param', 'value', 'reason',
    'question', 'options', 'result', 'task_message_no',
    'session_title', 'session_keywords',
    'summary_prompt', 'wait_ms', 'delta_x', 'delta_y',
    'table_index', 'after_para', 'same_style_as', 'style',
    'from_para', 'to_para', 'target',
    'paths', 'search', 'replace', 'replacements', 'wait', 'notes', 'backtick',
    # array item tag
    'item',
}

# Tags that are NOT tool/param tags and should NOT have prefix
IGNORED_TAGS = {
    # System info tags (environment_details)
    'system_info', 'os', 'arch', 'tool', 'shell', 'home', 'workspace', 'channel',
    # HTML/XML tags in tool descriptions or OpenAI mode examples
    'br', 'p', 'div', 'span', 'code', 'pre', 'em', 'strong', 'b', 'i', 'a',
    'ul', 'ol', 'li', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
    'table', 'tr', 'td', 'th', 'thead', 'tbody',
    'img', 'input', 'button', 'form', 'label', 'select', 'option', 'textarea',
    'xml', 'think', 'thinking', 'answer', 'result', 'analysis', 'reasoning',
    # Markdown/code fence references
    'backtick', 'note',
    # env details tags
    'environment_details', 'time', 'message_no', 'context_window', 'cwd', 'files', 'bin', 'research', 'task_plan', 'opened_resources', 'browser', 'excel', 'word', 'shell', 'session',
    'task',
}

def is_ignored_tag(tag):
    """Check if tag should be ignored (not a tool/param tag)."""
    if tag in IGNORED_TAGS:
        return True
    # Tags starting with these prefixes are not tool/param tags
    if tag.startswith('system_') or tag.startswith('key') or tag.startswith('func'):
        return True
    return False

def check_file(path):
    """Check a single i18n file for missing prefix on known tags."""
    with open(path) as f:
        content = f.read()
    
    missing = []
    prefixed = []
    
    # First, count ALL tags including prefixed ones.
    # Prefixed format: <{XML_TAG_PREFIX}tagname> or </{XML_TAG_PREFIX}tagname>
    # Bare format: <tagname> or </tagname>
    
    # Count prefixed tags: <{XML_TAG_PREFIX}, </{XML_TAG_PREFIX} (open and close)
    prefixed_count = len(re.findall(r'<\{XML_TAG_PREFIX\}', content))
    cs_count = len(re.findall(r'<cs:', content))
    
    # Find bare tags (without prefix) using regex that doesn't match prefixed ones
    for m in re.finditer(r'<(\w[\w-]*)>', content):
        tag = m.group(1)
        pos = m.start()
        
        # Skip ignored tags
        if is_ignored_tag(tag):
            continue
        
        line_num = content[:pos].count('\n') + 1
        
        if tag in KNOWN_TAGS:
            line_text = content[max(0,pos-40):pos+len(tag)+3]
            missing.append((tag, line_num, line_text))
    
    return missing, max(prefixed_count, cs_count)

def main():
    results = {}
    total_missing = 0
    
    for lang in ['zh', 'en']:
        path = f'i18n/{lang}_system.go'
        missing, prefixed = check_file(path)
        results[lang] = {'missing': missing, 'prefixed': prefixed}
        total_missing += len(missing)
        
        print(f"\n{'='*60}")
        print(f"File: {path}")
        print(f"Prefixed tags: {prefixed}")
        
        if missing:
            print(f"\n*** MISSING PREFIX ({len(missing)} tags): ***")
            by_tag = {}
            for tag, line, text in missing:
                by_tag.setdefault(tag, []).append((line, text))
            
            for tag in sorted(by_tag.keys()):
                occurrences = by_tag[tag]
                print(f"\n  <{tag}> — {len(occurrences)} occurrence(s):")
                for line, text in occurrences:
                    print(f"    Line {line}: ...{text.strip()}...")
        else:
            print(f"\n  ✅ ALL KNOWN TAGS HAVE PREFIX!")
    
    print(f"\n{'='*60}")
    print(f"TOTAL missing: {total_missing}")
    print(f"{'='*60}")
    
    if total_missing == 0:
        print("\n✅ PASS: All XML tool/param tags have the {XML_TAG_PREFIX} prefix!")
        return 0
    else:
        print(f"\n❌ FAIL: {total_missing} tags need prefix added.")
        return 1

if __name__ == '__main__':
    sys.exit(main())