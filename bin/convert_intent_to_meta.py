#!/usr/bin/env python3
"""
FEATURE-447: Convert top-level 'intent' parameter declarations in tools.go
to a unified 'meta' object parameter (placed first), and add 'instruct' to
vision tools (visual_analysis / browser_screenshot).

This operates on the tool definitions inside buildToolsInternal. It only
touches TOP-LEVEL tool parameters (the direct children of a tool's
"properties" map), NOT nested sub-object fields (e.g. the "intent" inside
replace_in_file's "replacements" items or update_settings' "settings" items).

The intent block may appear at different indentation levels (shell tools are
nested one level deeper). We match the intent block by its structural shape
regardless of leading whitespace, and only when it is a direct child of the
tool's "properties" map (i.e. the intent key is at the same indent as the
other top-level params like "path"/"command"/"session_id").
"""
import re
import sys

# A top-level intent property block. The key line indent is captured so we can
# emit the meta block at the same indent. The block is:
#   <indent>"intent": map[string]interface{}{{
#   <indent>\t"type":        "string",
#   <indent>\t"description": "...",
#   <indent>},
INTENT_BLOCK_RE = re.compile(
    r'(?P<indent>[ \t]*)"intent": map\[string\]interface\{\}\{\n'
    r'(?P=indent)\t"type":        "string",\n'
    r'(?P=indent)\t"description": (?:"[^"]*"|i18n\.T\(i18n\.KeySettingCmd_613\)),\n'
    r'(?P=indent)\},\n'
)

# The meta property block, emitted at a given indent.
def meta_prop(indent):
    return (
        indent + '"meta": map[string]interface{}{\n'
        + indent + '\t"type":        "object",\n'
        + indent + '\t"description": "**REQUIRED**: Transparency metadata object carrying intent/risk/risk_reason/files/progress. See the system prompt for the full structure.",\n'
        + indent + '},\n'
    )

def instruct_prop(indent):
    return (
        indent + '"instruct": map[string]interface{}{\n'
        + indent + '\t"type":        "string",\n'
        + indent + '\t"description": "**REQUIRED**: The explicit instruction for the vision model describing what to analyze/extract from the image(s). This is distinct from meta.intent (which is the intent shown to the user).",\n'
        + indent + '},\n'
    )


def convert_tool_block(block, tool_name):
    """Convert the intent param to meta in a single tool definition block."""
    m = INTENT_BLOCK_RE.search(block)
    if not m:
        return block, False
    indent = m.group('indent')
    meta = meta_prop(indent)
    if tool_name in ('visual_analysis', 'browser_screenshot'):
        meta += instruct_prop(indent)
    new_block = INTENT_BLOCK_RE.sub(meta, block, count=1)

    # Update the required list: replace "intent" with "meta" (and add "instruct").
    def fix_required(rm):
        inner = rm.group(1)
        items = [x.strip().strip('"') for x in inner.split(',')]
        items = [x for x in items if x != 'intent']
        new_items = ['meta'] + items
        if tool_name in ('visual_analysis', 'browser_screenshot') and 'instruct' not in new_items:
            new_items.append('instruct')
        quoted = ', '.join('"' + x + '"' for x in new_items)
        return '"required": []string{' + quoted + '}'
    new_block = re.sub(r'"required":\s*\[\]string\{([^}]*)\}', fix_required, new_block, count=1)
    return new_block, True


def main():
    filepath = sys.argv[1] if len(sys.argv) > 1 else 'agent/tools.go'
    with open(filepath, 'r', encoding='utf-8') as f:
        lines = f.read().split('\n')

    out = []
    i = 0
    changes = 0
    while i < len(lines):
        line = lines[i]
        m = re.match(r'(\s*)Name:\s+"(\w+)"', line)
        if m:
            tool_name = m.group(2)
            # Buffer the block from Name: to the matching Callback: line.
            j = i
            block_lines = []
            while j < len(lines):
                block_lines.append(lines[j])
                if re.match(r'\s*Callback:', lines[j]):
                    break
                j += 1
            block = '\n'.join(block_lines)
            new_block, ok = convert_tool_block(block, tool_name)
            if ok:
                changes += 1
                print(f"  [OK] {tool_name}")
            else:
                print(f"  [SKIP] {tool_name}")
            out.extend(new_block.split('\n'))
            i = j + 1
            continue
        out.append(line)
        i += 1

    with open(filepath, 'w', encoding='utf-8') as f:
        f.write('\n'.join(out))
    print(f"Converted {changes} tool definitions.")


if __name__ == '__main__':
    main()
