#!/usr/bin/env python3
"""Replace hardcoded cs: prefix with {XML_TAG_PREFIX} placeholder in i18n files."""
import re

for lang in ['zh', 'en']:
    path = f'i18n/{lang}_system.go'
    with open(path, 'r') as f:
        content = f.read()
    
    # Replace opening tags: <cs:tag -> <{XML_TAG_PREFIX}tag
    content = re.sub(r'<(cs:)([a-zA-Z_][a-zA-Z0-9_-]*)', r'<{XML_TAG_PREFIX}\2', content)
    # Replace closing tags: </cs:tag -> </{XML_TAG_PREFIX}tag
    content = re.sub(r'</(cs:)([a-zA-Z_][a-zA-Z0-9_-]*)', r'</{XML_TAG_PREFIX}\2', content)
    # Replace <cs_tool_calls> -> <{XML_TAG_PREFIX}tool_calls> in examples
    content = content.replace('<cs_tool_calls>', '<{XML_TAG_PREFIX}tool_calls>')
    content = content.replace('</cs_tool_calls>', '</{XML_TAG_PREFIX}tool_calls>')
    
    with open(path, 'w') as f:
        f.write(content)
    
    print(f'Updated {path}')