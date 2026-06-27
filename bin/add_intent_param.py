#!/usr/bin/env python3
"""
Add required 'intent' parameter to all LLM tool definitions in tools.go
that don't already have it.

Only add_images already has intent, so skip it.
"""
import re
import sys

def add_intent_to_tools(filepath):
    with open(filepath, 'r', encoding='utf-8') as f:
        lines = f.readlines()
    
    intent_prop_lines = [
        '\t\t\t\t\t"intent": map[string]interface{}{\n',
        '\t\t\t\t\t\t"type":        "string",\n',
        '\t\t\t\t\t\t"description": "**REQUIRED**: Explain why you are calling this tool and what you expect to accomplish. This helps track and debug LLM decision-making.",\n',
        '\t\t\t\t\t},\n',
    ]

    tool_name = None
    i = 0
    changes = 0
    
    while i < len(lines):
        line = lines[i]
        
        # Detect tool name
        m = re.match(r'(\s*)Name:\s+"(\w+)"', line)
        if m:
            tool_name = m.group(2)
        
        # Case 1: Empty properties - "properties": map[string]interface{}{}
        m1 = re.match(r'(\s*)"properties":\s*map\[string\]interface\{\}\{\},?\s*$', line)
        if m1 and tool_name != 'add_images':
            indent = m1.group(1)
            # Replace the empty properties line
            lines[i] = indent + '"properties": map[string]interface{}{\n'
            for pl in intent_prop_lines:
                lines.insert(i+1, pl)
                i += 1
            lines.insert(i+1, indent + '},\n')
            i += 1
            
            # Update required field (next line)
            if i+1 < len(lines):
                req_match = re.match(r'(\s*)"required":\s*\[\]string\{\},?\s*$', lines[i+1])
                if req_match:
                    req_indent = req_match.group(1)
                    lines[i+1] = req_indent + '"required": []string{"intent"},\n'
            changes += 1
            print(f"  [EMPTY] {tool_name}")
            i += 1
            continue
        
        # Case 2: Non-empty properties - "properties": map[string]interface{}{  (opening brace)
        m2 = re.match(r'(\s*)"properties":\s*map\[string\]interface\{\}\{$', line)
        if m2 and tool_name != 'add_images':
            # Find the indentation of existing properties
            # Next line should be the first property
            first_prop = i + 1
            if first_prop < len(lines):
                prop_indent_match = re.match(r'(\s*)', lines[first_prop])
                prop_indent = prop_indent_match.group(1) if prop_indent_match else '\t\t\t\t\t'
                
                # Create intent property with correct indentation
                intent_block = prop_indent + '"intent": map[string]interface{}{\n'
                intent_block += prop_indent + '\t"type":        "string",\n'
                intent_block += prop_indent + '\t"description": "**REQUIRED**: Explain why you are calling this tool and what you expect to accomplish. This helps track and debug LLM decision-making.",\n'
                intent_block += prop_indent + '},\n'
                
                # Insert intent property right after the opening brace
                lines.insert(first_prop, intent_block)
                
                # Find and update required field
                # Search within the next 10 lines for "required"
                for k in range(i+2, min(i+15, len(lines))):
                    req_match = re.match(r'(\s*)"required":\s*\[\]string\{([^}]*)\},?\s*$', lines[k])
                    if req_match:
                        req_indent = req_match.group(1)
                        existing = req_match.group(2).strip()
                        if existing:
                            lines[k] = req_indent + '"required": []string{"intent", ' + existing + '},\n'
                        else:
                            lines[k] = req_indent + '"required": []string{"intent"},\n'
                        break
                
                changes += 1
                print(f"  [HAS-PROPS] {tool_name}")
                i += 1  # Skip the inserted line
                continue
        
        i += 1
    
    with open(filepath, 'w', encoding='utf-8') as f:
        f.writelines(lines)
    
    print(f"\nTotal changes: {changes}")

if __name__ == '__main__':
    if len(sys.argv) > 1:
        add_intent_to_tools(sys.argv[1])
    else:
        add_intent_to_tools('/Users/direct3d/github/co-shell/agent/tools.go')
