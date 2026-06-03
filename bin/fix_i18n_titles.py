#!/usr/bin/env python3
"""Add section titles and ==== separators to i18n system prompt resources."""

import re
import sys

ZH_FILE = "i18n/zh_system.go"
EN_FILE = "i18n/en_system.go"

# (key_constant, title_to_insert)
SECTIONS = [
    ("KeySystemPromptToolUsageShell", "TOOL USE"),
    ("KeySystemPromptToolUsage", "TOOL USE"),
    ("KeySystemPromptResultMode", "RESULT MODE"),
    ("KeySystemPromptCapabilities", "CAPABILITIES"),
    ("KeySystemPromptCapabilitiesShell", "CAPABILITIES"),
    ("KeySystemPromptRules", "RULES"),
    ("KeySystemPromptRulesShell", "RULES"),
    ("KeySystemPromptObjective", "OBJECTIVE"),
    ("KeySystemPromptEnvironment", "SYSTEM INFORMATION"),
]


def process_file(path):
    with open(path, "r", encoding="utf-8") as f:
        content = f.read()

    for key, title in SECTIONS:
        # Pattern: zhMessages[KeyXxx] = `...` with content between backticks
        # We match the opening ` then find the next closing ` that starts at line start
        pattern = rf'({key}\s*=\s*`)(.*?)(`\s*\n)'
        
        def body_starts_with_title(body):
            stripped = body.lstrip()
            return stripped.startswith(title) or stripped.startswith(title + "\n")

        new_content, count = re.subn(pattern, lambda m: m.group(0) if body_starts_with_title(m.group(2)) else f'{m.group(1)}{title}\n\n{m.group(2)}{m.group(3)}', content, count=1)
        if count > 0:
            content = new_content
            print(f"  {path}: {key} <- {title}")
        else:
            print(f"  {path}: {key} NOT FOUND or already has title")

    with open(path, "w", encoding="utf-8") as f:
        f.write(content)


def main():
    print("Processing zh_system.go ...")
    process_file(ZH_FILE)
    print("Processing en_system.go ...")
    process_file(EN_FILE)
    print("Done.")


if __name__ == "__main__":
    main()