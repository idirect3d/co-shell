#!/usr/bin/env python3
"""Add cs: prefix to all XML tags in toolcall_mode_test.go test inputs."""
import re

with open('agent/toolcall_mode_test.go', 'r') as f:
    content = f.read()

def add_prefix_to_line(line):
    """Add cs: prefix to all XML tags in a test input line."""
    # Replace opening tags: <tagname -> <cs:tagname (but not </, <!--, <![CDATA[, <cs:)
    line = re.sub(
        r'<([a-zA-Z_][a-zA-Z0-9_-]*)',
        lambda m: '<cs:' + m.group(1) if not m.group(0).startswith('</') and not m.group(0).startswith('<!--') and not m.group(0).startswith('<![CDATA[') and not m.group(0).startswith('<cs:') and not m.group(0).startswith('<cs_tool_calls') else m.group(0),
        line
    )
    # Replace closing tags: </tagname -> </cs:tagname
    line = re.sub(
        r'</([a-zA-Z_][a-zA-Z0-9_-]*)',
        lambda m: '</cs:' + m.group(1) if not m.group(1).startswith('cs') and not m.group(0).startswith('</cs:') and not m.group(0).startswith('</cs_tool_calls') else m.group(0),
        line
    )
    return line

lines = content.split('\n')
output_lines = []
for line in lines:
    if ('xmlInput := "' in line or 'xmlInput := `' in line) and '<' in line:
        line = add_prefix_to_line(line)
    output_lines.append(line)

output = '\n'.join(output_lines)

with open('agent/toolcall_mode_test.go', 'w') as f:
    f.write(output)

print("Done - prefixes added to xmlInput lines")