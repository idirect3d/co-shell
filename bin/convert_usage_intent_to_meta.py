#!/usr/bin/env python3
"""
FEATURE-447: Update XML tool usage templates (i18n KeyToolUsage*) to use the
unified meta object parameter instead of the top-level intent parameter.

For each KeyToolUsage* template in en_system.go / zh_system.go:
1. Replace the "- intent (required) ..." parameter line with a "- meta (required) ..."
   object parameter line.
2. Remove any residual "- intent (required)..." parameter lines (some templates
   have a nested/indented intent line or a colon variant that the first pass
   missed).
3. Replace the XML usage example "<intent>...</intent>" with a "<meta>...</meta>"
   object containing intent + risk.
4. For vision tools (visual_analysis / browser_screenshot), add an instruct
   parameter line and an <instruct> XML element.
"""
import re
import sys

# The intent parameter line (en and zh variants), possibly indented, possibly
# with a colon after "(required)".
INTENT_PARAM_EN = re.compile(r'^[ \t]*- intent \(required\)[:\s].*$', re.MULTILINE)
INTENT_PARAM_ZH = re.compile(r'^[ \t]*- intent \(必需\)[:\s].*$', re.MULTILINE)

META_PARAM_EN = '- meta (required) Transparency metadata object carrying intent/risk/risk_reason/files/progress. See the system prompt for the full structure.'
META_PARAM_ZH = '- meta (必需) 透明元数据对象，包含 intent/risk/risk_reason/files/progress。完整结构见系统提示词。'

INSTRUCT_PARAM_EN = '- instruct (required) The explicit instruction for the vision model describing what to analyze/extract from the image(s). Distinct from meta.intent (which is the intent shown to the user).'
INSTRUCT_PARAM_ZH = '- instruct (必需) 给视觉模型的明确指令，描述要从图像中分析/提取什么。与 meta.intent（展示给用户的意图）不同。'

# XML usage example: <prefix:intent>...</prefix:intent>
XML_INTENT_RE = re.compile(
    r'<\{XML_TAG_PREFIX\}intent>(.*?)</\{XML_TAG_PREFIX\}intent>',
    re.DOTALL,
)

# Vision tools that need an instruct element.
VISION_TOOLS = {'visual_analysis', 'browser_screenshot'}


def process_template(content, is_zh, tool_name):
    """Convert intent to meta in a single KeyToolUsage* template body."""
    # 1. Remove ALL residual intent parameter lines (indented or colon variants).
    if is_zh:
        content = INTENT_PARAM_ZH.subn('', content)[0]
    else:
        content = INTENT_PARAM_EN.subn('', content)[0]

    # 2. Ensure a meta parameter line exists right after "Parameters:".
    meta_line = META_PARAM_ZH if is_zh else META_PARAM_EN
    if meta_line not in content:
        content = re.sub(
            r'(Parameters:\n)',
            r'\1' + meta_line + '\n',
            content, count=1)

    # 3. Replace the XML <intent> element with a <meta> object.
    def xml_meta(m):
        inner = m.group(1).strip()
        return ('<{XML_TAG_PREFIX}meta>\n'
                '  <{XML_TAG_PREFIX}intent>' + inner + '</{XML_TAG_PREFIX}intent>\n'
                '  <{XML_TAG_PREFIX}risk>low</{XML_TAG_PREFIX}risk>\n'
                '</{XML_TAG_PREFIX}meta>')
    content = XML_INTENT_RE.sub(xml_meta, content)

    # 4. For vision tools, add instruct param line and <instruct> element.
    if tool_name in VISION_TOOLS:
        # Add instruct param line after the meta param line.
        content = content.replace(meta_line + '\n',
                                  meta_line + '\n' + (INSTRUCT_PARAM_ZH if is_zh else INSTRUCT_PARAM_EN) + '\n', 1)
        # Add <instruct> element inside the <meta> object (after risk).
        content = content.replace(
            '  <{XML_TAG_PREFIX}risk>low</{XML_TAG_PREFIX}risk>\n'
            '</{XML_TAG_PREFIX}meta>',
            '  <{XML_TAG_PREFIX}risk>low</{XML_TAG_PREFIX}risk>\n'
            '  <{XML_TAG_PREFIX}instruct>Describe what to analyze in the image</{XML_TAG_PREFIX}instruct>\n'
            '</{XML_TAG_PREFIX}meta>', 1)

    return content


def main():
    filepath = sys.argv[1]
    is_zh = 'zh' in filepath
    with open(filepath, 'r', encoding='utf-8') as f:
        content = f.read()

    def process_block(m):
        key = m.group(2)
        body = m.group(3)
        tool_name = key.replace('KeyToolUsage', '')
        import re as _re
        snake = _re.sub(r'(?<!^)(?=[A-Z])', '_', tool_name).lower()
        new_body = process_template(body, is_zh, snake)
        return m.group(0).replace(body, new_body)

    pattern = re.compile(r'(enMessages|zhMessages)\[(KeyToolUsage\w+)\] = `(.*?)`', re.DOTALL)
    new_content, count = pattern.subn(process_block, content)

    with open(filepath, 'w', encoding='utf-8') as f:
        f.write(new_content)
    print(f"Processed {count} KeyToolUsage templates in {filepath}")


if __name__ == '__main__':
    main()
