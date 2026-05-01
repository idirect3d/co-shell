#!/usr/bin/env python3
"""
md2wechat.py — Markdown to WeChat Official Account Format Converter

Converts Markdown files into HTML suitable for pasting into the WeChat
Official Account editor. Handles headings, tables, code blocks, blockquotes,
bold, italic, links, lists, and horizontal rules.

Usage:
    python3 md2wechat.py input.md [output.html]

If output file is not specified, prints to stdout.
"""

import sys
import re
import html as html_module


def convert(md_text):
    """Convert Markdown text to WeChat-friendly HTML."""
    lines = md_text.split('\n')
    html_parts = []
    i = 0
    n = len(lines)

    while i < n:
        line = lines[i]

        # --- Horizontal rule ---
        if re.match(r'^---+\s*$', line) or re.match(r'^\*\*\*+\s*$', line):
            html_parts.append('<hr style="border: none; border-top: 1px solid #ddd; margin: 20px 0;">')
            i += 1
            continue

        # --- Code block (```) ---
        if line.startswith('```'):
            lang = line[3:].strip()
            code_lines = []
            i += 1
            while i < n and not lines[i].startswith('```'):
                code_lines.append(lines[i])
                i += 1
            i += 1  # skip closing ```
            code_text = html_module.escape('\n'.join(code_lines))
            html_parts.append(
                '<pre style="background-color: #f5f5f5; border: 1px solid #ddd; '
                'border-radius: 4px; padding: 12px; font-size: 13px; '
                'line-height: 1.6; overflow-x: auto; white-space: pre-wrap; '
                'word-break: break-all; font-family: Menlo, Monaco, Consolas, monospace;">'
                f'{code_text}</pre>'
            )
            continue

        # --- Blockquote ---
        if line.startswith('> '):
            quote_lines = []
            while i < n and lines[i].startswith('> '):
                quote_lines.append(lines[i][2:])
                i += 1
            quote_text = '<br>'.join(quote_lines)
            html_parts.append(
                '<blockquote style="border-left: 4px solid #07c160; '
                'background-color: #f0f9f4; padding: 10px 15px; margin: 10px 0; '
                'color: #333; font-size: 14px; line-height: 1.8;">'
                f'{inline_convert(quote_text)}</blockquote>'
            )
            continue

        # --- Table ---
        if '|' in line and i + 1 < n and re.match(r'^\|[\s\-:|]+\|$', lines[i + 1]):
            table_html = ['<table style="border-collapse: collapse; width: 100%; margin: 10px 0; font-size: 14px;">']
            # Header row
            headers = [h.strip() for h in line.split('|') if h.strip()]
            table_html.append('<thead><tr>')
            for h in headers:
                table_html.append(
                    f'<th style="border: 1px solid #ddd; padding: 8px 12px; '
                    f'background-color: #07c160; color: white; text-align: center; '
                    f'font-weight: bold;">{inline_convert(h)}</th>'
                )
            table_html.append('</tr></thead>')
            i += 2  # skip header and separator
            table_html.append('<tbody>')
            while i < n and '|' in lines[i]:
                cells = [c.strip() for c in lines[i].split('|') if c.strip()]
                table_html.append('<tr>')
                for c in cells:
                    table_html.append(
                        f'<td style="border: 1px solid #ddd; padding: 8px 12px; '
                        f'text-align: center; font-size: 14px;">{inline_convert(c)}</td>'
                    )
                table_html.append('</tr>')
                i += 1
            table_html.append('</tbody></table>')
            html_parts.append('\n'.join(table_html))
            continue

        # --- Unordered list ---
        if re.match(r'^[\-\*]\s', line):
            list_items = []
            while i < n and re.match(r'^[\-\*]\s', lines[i]):
                item_text = re.sub(r'^[\-\*]\s', '', lines[i])
                list_items.append(f'<li>{inline_convert(item_text)}</li>')
                i += 1
            html_parts.append(
                '<ul style="padding-left: 20px; margin: 8px 0; font-size: 14px; line-height: 1.8;">'
                f'{"".join(list_items)}</ul>'
            )
            continue

        # --- Ordered list ---
        if re.match(r'^\d+\.\s', line):
            list_items = []
            while i < n and re.match(r'^\d+\.\s', lines[i]):
                item_text = re.sub(r'^\d+\.\s', '', lines[i])
                list_items.append(f'<li>{inline_convert(item_text)}</li>')
                i += 1
            html_parts.append(
                '<ol style="padding-left: 20px; margin: 8px 0; font-size: 14px; line-height: 1.8;">'
                f'{"".join(list_items)}</ol>'
            )
            continue

        # --- Headings ---
        heading_match = re.match(r'^(#{1,6})\s+(.+)$', line)
        if heading_match:
            level = len(heading_match.group(1))
            text = heading_match.group(2)
            font_sizes = {1: '22px', 2: '18px', 3: '16px', 4: '15px', 5: '14px', 6: '14px'}
            html_parts.append(
                f'<h{level} style="font-size: {font_sizes.get(level, "16px")}; '
                f'font-weight: bold; margin: 15px 0 10px 0; line-height: 1.6;">'
                f'{inline_convert(text)}</h{level}>'
            )
            i += 1
            continue

        # --- Empty line ---
        if not line.strip():
            html_parts.append('<p style="margin: 5px 0;">&nbsp;</p>')
            i += 1
            continue

        # --- Regular paragraph ---
        html_parts.append(
            f'<p style="font-size: 14px; line-height: 1.8; margin: 8px 0; '
            f'text-indent: 0;">{inline_convert(line)}</p>'
        )
        i += 1

    return '\n'.join(html_parts)


def inline_convert(text):
    """Convert inline Markdown formatting to HTML."""
    # Bold + Italic ***text***
    text = re.sub(r'\*\*\*(.+?)\*\*\*', r'<strong><em>\1</em></strong>', text)
    # Bold **text**
    text = re.sub(r'\*\*(.+?)\*\*', r'<strong>\1</strong>', text)
    # Italic *text*
    text = re.sub(r'\*(.+?)\*', r'<em>\1</em>', text)
    # Inline code `text`
    text = re.sub(r'`([^`]+)`', r'<code style="background-color: #f5f5f5; padding: 2px 4px; border-radius: 3px; font-size: 13px; font-family: Menlo, Monaco, Consolas, monospace;">\1</code>', text)
    # Links [text](url)
    text = re.sub(
        r'\[([^\]]+)\]\(([^)]+)\)',
        r'<a href="\2" style="color: #07c160; text-decoration: none;">\1</a>',
        text
    )
    return text


def main():
    if len(sys.argv) < 2:
        print("Usage: python3 md2wechat.py input.md [output.html]", file=sys.stderr)
        sys.exit(1)

    input_path = sys.argv[1]
    output_path = sys.argv[2] if len(sys.argv) > 2 else None

    with open(input_path, 'r', encoding='utf-8') as f:
        md_text = f.read()

    html = convert(md_text)

    # Wrap in a container div with WeChat-friendly base styles
    full_html = (
        '<div style="max-width: 677px; margin: 0 auto; padding: 10px 15px; '
        'font-family: -apple-system, BlinkMacSystemFont, \'Helvetica Neue\', '
        '\'PingFang SC\', \'Microsoft YaHei\', sans-serif; color: #333;">\n'
        f'{html}\n'
        '</div>'
    )

    if output_path:
        with open(output_path, 'w', encoding='utf-8') as f:
            f.write(full_html)
        print(f"Converted: {input_path} -> {output_path}", file=sys.stderr)
    else:
        print(full_html)


if __name__ == '__main__':
    main()
