#!/usr/bin/env python3
"""Replace ALL XML tool/param tags with {XML_TAG_PREFIX} placeholder in i18n files.
This adds the prefix to ALL tags inside XML Usage examples that represent
tool names or parameter names (not HTML or other non-tool content)."""
import re

for lang in ['zh', 'en']:
    path = f'i18n/{lang}_system.go'
    with open(path, 'r') as f:
        content = f.read()
    
    # Phase 1: Replace already-hardcoded cs: prefix tags
    # <cs:tag -> <{XML_TAG_PREFIX}tag
    content = re.sub(r'<(cs:)([a-zA-Z_][a-zA-Z0-9_-]*)', r'<{XML_TAG_PREFIX}\2', content)
    # </cs:tag -> </{XML_TAG_PREFIX}tag  
    content = re.sub(r'</(cs:)([a-zA-Z_][a-zA-Z0-9_-]*)', r'</{XML_TAG_PREFIX}\2', content)
    
    # Phase 2: Add prefix to bare XML tags that appear in Usage examples.
    # These are inside Go string literals in the system prompt files.
    # We target tags that look like tool names or parameter names:
    # common tool names: execute_command, read_file, search_files, write_to_file, etc.
    # common param names: command, intent, path, content, mode, etc.
    # We add prefix to ALL <word> tags since in XML mode context there should be no bare tags.
    #
    # Strategy: In Go string literals (backtick-quoted raw strings), replace
    # opening tags <word> with <{XML_TAG_PREFIX}word> and closing tags </word>
    # with </{XML_TAG_PREFIX}word>, but only for tags that look like tool/param 
    # identifiers (letters, digits, underscores).
    
    # Define the set of known tool and parameter tag names to prefix.
    # These appear in the XML Usage examples.
    known_tags = [
        'execute_command', 'read_file', 'search_files', 'write_to_file', 
        'replace_in_file', 'list_files', 'list_code_definition_names',
        'visual_analysis', 'launch_sub_agent', 'schedule_task',
        'track_task_progress', 'view_task_plan',
        'get_memory_slice', 'memory_search', 'delete_memory',
        'update_settings', 'list_settings',
        'ask_followup_question', 'attempt_completion',
        'evaluate_expression', 'reorganize_context',
        'shell_send', 'shell_get_output', 'shell_window_content', 'shell_reset',
        'browser_navigate', 'browser_screenshot', 'browser_click',
        'browser_type', 'browser_evaluate', 'browser_get_rendered_html',
        'browser_scroll', 'browser_get_interactive_elements',
        'browser_go_back', 'browser_go_forward', 'browser_close',
        'vault_list', 'vault_add', 'vault_remove',
        'word_open', 'word_close', 'word_save', 'word_overview',
        'word_read', 'word_table_read', 'word_continue', 'word_erase',
        'word_inspect_style', 'word_format',
        'excel_open', 'excel_close', 'excel_save', 'excel_overview',
        'excel_read', 'excel_edit', 'excel_copy', 'excel_paste',
        'excel_insert', 'excel_delete', 'excel_sheet', 'excel_format',
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
        'paths', 'search', 'replace', 'replacements',
    ]
    known_set = set(known_tags)
    
    # Scan for tags inside backtick-quoted strings (Go raw string literals)
    # We process line by line within the strings
    in_backtick = False
    backtick_content = []
    backtick_lines = []
    result_lines = content.split('\n')
    new_lines = []
    
    i = 0
    while i < len(result_lines):
        line = result_lines[i]
        # Check for start/end of backtick string
        backtick_start = line.count('`') > 0 and not in_backtick
        backtick_end = line.count('`') > 0 and in_backtick
        
        if '`' in line and not in_backtick:
            # Start of backtick string
            idx = line.index('`')
            before = line[:idx+1]
            after = line[idx+1:]
            in_backtick = True
            backtick_content = []
            
            # Check if there's another backtick closing on the same line
            if after.count('`') > 0:
                # Multi-line string all on one line
                end_idx = after.index('`')
                content = after[:end_idx]
                after_rest = after[end_idx:]
                # Process content
                processed = process_xml_tags(content, known_set)
                line = before + processed + after_rest
                in_backtick = False
                new_lines.append(line)
            else:
                # Content continues
                backtick_content.append(after)
                new_lines.append(line)  # Keep the opening line
        elif '`' in line and in_backtick:
            # End of backtick string
            idx = line.index('`')
            before = line[:idx]
            after = line[idx+1:]
            in_backtick = False
            backtick_content.append(before)
            combined = '\n'.join(backtick_content)
            processed = process_xml_tags(combined, known_set)
            backtick_lines = processed.split('\n')
            
            # Replace the backtick content lines
            # We've already added the opening line, now we need to
            # remove the intermediate lines and add processed ones
            # Actually this is complex. Let me use a simpler approach.
            new_lines.append(line)  # Keep ending line as-is
        else:
            new_lines.append(line)
        i += 1
    
    # If we still had unclosed backtick, process it
    if in_backtick and backtick_content:
        combined = '\n'.join(backtick_content)
        processed = process_xml_tags(combined, known_set)
        # Replace the last lines
        # This is getting complex... let me use a simpler regex approach instead
    
    # SIMPLER APPROACH: just do regex on the whole file content
    # replacing ALL XML tags that look like identifiers
    # This is safe because in XML mode context, all tags should have the prefix
    
    with open(path, 'r') as f:
        content = f.read()
    
    # Process phase by phase:
    # 1. First, protect already-prefixed tags
    # 2. Then add prefix to remaining un-prefixed tags
    
    # Protected patterns (already have prefix or are in OpenAI mode context)
    protected = re.compile(r'<\{XML_TAG_PREFIX\}|<![CDATA\[|<!--')
    
    # Replace opening tags: <tagname> where tagname is a word
    # Skip if already protected or is a closing/comment tag
    def add_prefix_opening(m):
        full = m.group(0)
        tag = m.group(1)
        # Skip if already has prefix, is closing tag, is comment, is CDATA
        if full.startswith('<{') or full.startswith('</') or full.startswith('<!--') or full.startswith('<![CDATA['):
            return full
        # Only add prefix for known tool/param tags
        if tag in known_set:
            return '<{XML_TAG_PREFIX}' + tag + '>'
        return full
    
    content = re.sub(r'<(\w[\w-]*)>', add_prefix_opening, content)
    
    # Replace closing tags: </tagname>
    def add_prefix_closing(m):
        full = m.group(0)
        tag = m.group(1)
        if full.startswith('</{') or full.startswith('</cs:'):
            return full
        if tag in known_set:
            return '</{XML_TAG_PREFIX}' + tag + '>'
        return full
    
    content = re.sub(r'</(\w[\w-]*)>', add_prefix_closing, content)
    
    with open(path, 'w') as f:
        f.write(content)
    
    print(f'Updated {path}')

def process_xml_tags(text, known_set):
    """Replace bare XML tags with prefixed versions."""
    # Opening tags
    def repl_open(m):
        tag = m.group(1)
        if tag in known_set:
            return '<{XML_TAG_PREFIX}' + tag + '>'
        return m.group(0)
    
    def repl_close(m):
        tag = m.group(1)
        if tag in known_set:
            return '</{XML_TAG_PREFIX}' + tag + '>'
        return m.group(0)
    
    text = re.sub(r'<(\w[\w-]*)>', repl_open, text)
    text = re.sub(r'</(\w[\w-]*)>', repl_close, text)
    return text