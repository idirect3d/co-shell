#!/usr/bin/env python3
"""Remove standalone ==== lines from i18n section content.
These will now be added programmatically by buildSystemPromptWithMode()."""

import re

for fn in ['i18n/zh_system.go']:
    with open(fn) as f:
        lines = f.readlines()
    
    result = []
    i = 0
    removed = 0
    while i < len(lines):
        line = lines[i]
        # Remove a line that is ONLY "====\n" (standalone separator)
        if re.match(r'^====\n$', line):
            removed += 1
            i += 1
            continue
        result.append(line)
        i += 1
    
    print(f"{fn}: removed {removed} separator lines")
    with open(fn, 'w') as f:
        f.writelines(result)