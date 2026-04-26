#!/usr/bin/env python3
"""Markdown to Beautiful Word Document Converter.

Usage:
    python3 md2docx.py input.md -o output.docx
    python3 md2docx.py input.md --style modern
"""

import argparse
import os
import re
import sys
from datetime import datetime

from docx import Document
from docx.shared import Inches, Pt, Cm, RGBColor
from docx.enum.text import WD_ALIGN_PARAGRAPH
from docx.enum.table import WD_TABLE_ALIGNMENT
from docx.oxml.ns import qn, nsdecls
from docx.oxml import parse_xml


def _rgb(r, g, b):
    """Create RGBColor."""
    return RGBColor(r, g, b)


STYLES = {
    "modern": {
        "primary": _rgb(0x1A, 0x73, 0xE8),
        "secondary": _rgb(0x5F, 0x63, 0x68),
        "heading1": _rgb(0x1A, 0x73, 0xE8),
        "heading2": _rgb(0x19, 0x60, 0x7D),
        "heading3": _rgb(0x37, 0x47, 0x4F),
        "body": _rgb(0x33, 0x33, 0x33),
        "code_bg": "E8F0FE",
        "code_fg": _rgb(0x1D, 0x1D, 0x1D),
        "link": _rgb(0x1A, 0x0D, 0xAB),
        "blockquote_bg": "E8EAF6",
        "table_header": "1A73E8",
        "table_alt_row": "F1F3F4",
    },
    "classic": {
        "primary": _rgb(0x2C, 0x3E, 0x50),
        "secondary": _rgb(0x7F, 0x8C, 0x8D),
        "heading1": _rgb(0x2C, 0x3E, 0x50),
        "heading2": _rgb(0x2C, 0x3E, 0x50),
        "heading3": _rgb(0x7F, 0x8C, 0x8D),
        "body": _rgb(0x1A, 0x1A, 0x1A),
        "code_bg": "ECF0F1",
        "code_fg": _rgb(0x2C, 0x3E, 0x50),
        "link": _rgb(0x29, 0x80, 0xB9),
        "blockquote_bg": "F5F5F5",
        "table_header": "2C3E50",
        "table_alt_row": "F8F9FA",
    },
    "minimal": {
        "primary": _rgb(0x00, 0x00, 0x00),
        "secondary": _rgb(0x55, 0x55, 0x55),
        "heading1": _rgb(0x00, 0x00, 0x00),
        "heading2": _rgb(0x33, 0x33, 0x33),
        "heading3": _rgb(0x55, 0x55, 0x55),
        "body": _rgb(0x33, 0x33, 0x33),
        "code_bg": "F5F5F5",
        "code_fg": _rgb(0x33, 0x33, 0x33),
        "link": _rgb(0x55, 0x55, 0x55),
        "blockquote_bg": "FAFAFA",
        "table_header": "333333",
        "table_alt_row": "F5F5F5",
    },
}

HEADING_SIZES = {1: 22, 2: 18, 3: 15, 4: 13, 5: 12, 6: 11}


def set_cell_shading(cell, color_hex):
    """Set table cell background color."""
    shading = parse_xml('<w:shd ' + nsdecls('w') + ' w:fill="' + color_hex + '"/>')
    cell._tc.get_or_add_tcPr().append(shading)


def set_spacing(para, before=0, after=0, line=None):
    """Set paragraph spacing."""
    pf = para.paragraph_format
    pf.space_before = Pt(before)
    pf.space_after = Pt(after)
    if line:
        pf.line_spacing = line


def add_run(paragraph, text, bold=False, italic=False, color=None,
            size=None, font_name=None, underline=False):
    """Add a styled run to a paragraph."""
    run = paragraph.add_run(text)
    if bold: run.bold = True
    if italic: run.italic = True
    if color: run.font.color.rgb = color
    if size: run.font.size = Pt(size)
    if font_name: run.font.name = font_name
    if underline: run.font.underline = True
    return run


def color_to_hex(c):
    """Convert RGBColor to hex string."""
    return '%02X%02X%02X' % (c[0], c[1], c[2])


def add_bottom_border(para, color_hex, sz=6):
    """Add bottom border to paragraph."""
    text = '<w:pBdr ' + nsdecls('w') + '>'
    text += '  <w:bottom w:val="single" w:sz="' + str(sz) + '" w:space="4" w:color="' + color_hex + '"/>'
    text += '</w:pBdr>'
    para._p.get_or_add_pPr().append(parse_xml(text))


def add_left_border(para, color_hex, sz=18):
    """Add left border to paragraph."""
    text = '<w:pBdr ' + nsdecls('w') + '>'
    text += '  <w:left w:val="single" w:sz="' + str(sz) + '" w:space="8" w:color="' + color_hex + '"/>'
    text += '</w:pBdr>'
    para._p.get_or_add_pPr().append(parse_xml(text))


def add_shading(para, color_hex):
    """Add background shading."""
    text = '<w:shd ' + nsdecls('w') + ' w:fill="' + color_hex + '" w:val="clear"/>'
    para._p.get_or_add_pPr().append(parse_xml(text))


def add_table_borders(table):
    """Add borders to a table."""
    tbl = table._tbl
    tblPr = tbl.tblPr
    if tblPr is None:
        tblPr = parse_xml('<w:tblPr ' + nsdecls('w') + '/>')
        tbl.insert(0, tblPr)
    text = '<w:tblBorders ' + nsdecls('w') + '>'
    text += '<w:top w:val="single" w:sz="4" w:space="0" w:color="999999"/>'
    text += '<w:left w:val="single" w:sz="4" w:space="0" w:color="999999"/>'
    text += '<w:bottom w:val="single" w:sz="4" w:space="0" w:color="999999"/>'
    text += '<w:right w:val="single" w:sz="4" w:space="0" w:color="999999"/>'
    text += '<w:insideH w:val="single" w:sz="4" w:space="0" w:color="999999"/>'
    text += '<w:insideV w:val="single" w:sz="4" w:space="0" w:color="999999"/>'
    text += '</w:tblBorders>'
    tblPr.append(parse_xml(text))


def parse_inline(text):
    """Parse markdown inline formatting.
    
    Supports: **bold**, *italic*, `code`, [link](url)
    Returns list of (text, properties_dict).
    """
    tokens = []
    pattern = r'(\*\*(.+?)\*\*|\*(.+?)\*|`(.+?)`|\[(.+?)\]\((.+?)\))'
    last_end = 0
    for m in re.finditer(pattern, text):
        start, end = m.start(), m.end()
        if start > last_end:
            tokens.append((text[last_end:start], {}))
        full = m.group(0)
        if full.startswith('**'):
            tokens.append((m.group(2), {'bold': True}))
        elif full.startswith('*'):
            tokens.append((m.group(3), {'italic': True}))
        elif full.startswith('`'):
            tokens.append((m.group(4), {'code': True}))
        elif full.startswith('['):
            tokens.append((m.group(5), {'link': m.group(6)}))
        last_end = end
    if last_end < len(text):
        tokens.append((text[last_end:], {}))
    return tokens if tokens else [(text, {})]


def apply_inline(paragraph, text, style_name, font_size=11, font_family='Arial'):
    """Apply inline markdown formatting to a paragraph."""
    s = STYLES[style_name]
    for token_text, props in parse_inline(text):
        if not token_text:
            continue
        run = paragraph.add_run(token_text)
        run.font.name = font_family
        run.font.size = Pt(font_size)
        if props.get('bold'):
            run.bold = True
        elif props.get('italic'):
            run.italic = True
        elif props.get('code'):
            run.font.name = 'Courier New'
            run.font.color.rgb = s['code_fg']
        elif props.get('link'):
            run.font.color.rgb = s['link']
            run.font.underline = True
        else:
            run.font.color.rgb = s['body']


def build_cover_page(doc, title, author, style_name):
    """Build a cover page."""
    s = STYLES[style_name]
    for _ in range(6):
        doc.add_paragraph()
    p = doc.add_paragraph()
    p.alignment = WD_ALIGN_PARAGRAPH.CENTER
    add_run(p, '\u2501' * 40, color=s['primary'], size=12)
    p = doc.add_paragraph()
    p.alignment = WD_ALIGN_PARAGRAPH.CENTER
    add_run(p, title, bold=True, color=s['primary'], size=28, font_name='Arial')
    p = doc.add_paragraph()
    p.alignment = WD_ALIGN_PARAGRAPH.CENTER
    add_run(p, '\u2501' * 40, color=s['primary'], size=12)
    p = doc.add_paragraph()
    p.alignment = WD_ALIGN_PARAGRAPH.CENTER
    add_run(p, 'By ' + author, color=s['secondary'], size=12)
    p = doc.add_paragraph()
    p.alignment = WD_ALIGN_PARAGRAPH.CENTER
    add_run(p, datetime.now().strftime('%B %d, %Y'), color=s['secondary'], size=11)
    doc.add_page_break()


def build_toc(doc, headings, style_name):
    """Build a table of contents."""
    s = STYLES[style_name]
    p = doc.add_paragraph()
    add_run(p, 'Table of Contents', bold=True, color=s['primary'], size=18)
    set_spacing(p, after=12)
    p = doc.add_paragraph()
    add_run(p, '\u2501' * 60, color=s['secondary'], size=8)
    set_spacing(p, after=6)
    for level, text in headings:
        indent = '    ' * (level - 1)
        bullets = {1: '\u25CF', 2: '\u25CB', 3: '\u25AA'}
        bullet = bullets.get(level, '\u2022')
        sz = 12 - level if level <= 2 else 10
        p = doc.add_paragraph()
        add_run(p, indent + bullet + '  ' + text,
                color=s['body'] if level <= 3 else s['secondary'],
                size=max(sz, 9))
        set_spacing(p, before=2, after=2)
    doc.add_page_break()


def build_heading(doc, text, level, style_name, base_size):
    """Build a styled heading."""
    s = STYLES[style_name]
    colors = {1: s['heading1'], 2: s['heading2'], 3: s['heading3']}
    color = colors.get(level, s['body'])
    sz = HEADING_SIZES.get(level, base_size)
    p = doc.add_paragraph()
    add_run(p, text, bold=True, color=color, size=sz, font_name='Arial')
    if level <= 2:
        add_bottom_border(p, color_to_hex(color))
    sb = {1: 24, 2: 18, 3: 12, 4: 8, 5: 6, 6: 6}
    sa = {1: 12, 2: 8, 3: 6, 4: 4, 5: 3, 6: 3}
    set_spacing(p, before=sb.get(level, ), after=sa.get(level, 4))


def build_code_block(doc, code_text, style_name, font_size):
    """Build a styled code block."""
    s = STYLES[style_name]
    table = doc.add_table(rows=1, cols=1)
    table.alignment = WD_TABLE_ALIGNMENT.LEFT
    cell = table.cell(0, 0)
    set_cell_shading(cell, s['code_bg'])
    cell.paragraphs[0].clear()
    for cl in code_text.split('\n'):
        p = cell.add_paragraph()
        run = p.add_run(cl if cl else ' ')
        run.font.name = 'Courier New'
        run.font.size = Pt(font_size)
        run.font.color.rgb = s['code_fg']
        set_spacing(p, before=0, after=0, line=1.2)
    if cell.paragraphs[0].text.strip() == '':
        cell.paragraphs[0]._p.getparent().remove(cell.paragraphs[0]._p)
    doc.add_paragraph()


def build_blockquote(doc, text, style_name, font_size):
    """Build a styled blockquote."""
    s = STYLES[style_name]
    p = doc.add_paragraph()
    apply_inline(p, text, style_name, font_size)
    add_left_border(p, color_to_hex(s['primary']))
    p.paragraph_format.left_indent = Cm(1.0)
    add_shading(p, s['blockquote_bg'])
    set_spacing(p, before=6, after=6)


def build_list(doc, items, ordered, style_name, font_size):
    """Build a styled list."""
    s = STYLES[style_name]
    for idx, item_text in enumerate(items):
        p = doc.add_paragraph()
        prefix = str(idx + 1) + '. ' if ordered else '\u2022 '
        run = p.add_run(prefix)
        run.bold = True
        run.font.color.rgb = s['primary']
        run.font.size = Pt(font_size)
        run.font.name = 'Arial'
        apply_inline(p, item_text, style_name, font_size)
        pf = p.paragraph_format
        pf.left_indent = Cm(1.0)
        pf.first_line_indent = Cm(-0.5)
        set_spacing(p, before=2, after=2)


def build_table(doc, data, style_name, font_size):
    """Build a styled table."""
    s = STYLES[style_name]
    if not data or not data[0]:
        return
    rows = len(data)
    cols = len(data[0])
    table = doc.add_table(rows=rows, cols=cols)
    table.alignment = WD_TABLE_ALIGNMENT.CENTER
    for j, ct in enumerate(data[0]):
        cell = table.cell(0, j)
        set_cell_shading(cell, s['table_header'])
        p = cell.paragraphs[0]
        p.alignment = WD_ALIGN_PARAGRAPH.CENTER
        run = p.add_run(ct)
        run.bold = True
        run.font.color.rgb = RGBColor(0xFF, 0xFF, 0xFF)
        run.font.size = Pt(font_size)
        run.font.name = 'Arial'
    for i in range(1, rows):
        for j in range(cols):
            cell = table.cell(i, j)
            if i % 2 == 0:
                set_cell_shading(cell, s['table_alt_row'])
            text = data[i][j] if j < len(data[i]) else ''
            p = cell.paragraphs[0]
            apply_inline(p, text, style_name, max(font_size - 1, 9))
    add_table_borders(table)
    doc.add_paragraph()


def build_image(doc, img_path, alt_text, style_name, input_dir):
    """Build an image element."""
    s = STYLES[style_name]
    if not os.path.isabs(img_path):
        img_path = os.path.join(input_dir, img_path)
    if os.path.exists(img_path):
        try:
            p = doc.add_paragraph()
            p.alignment = WD_ALIGN_PARAGRAPH.CENTER
            run = p.add_run()
            run.add_picture(img_path, width=Inches(4.5))
            set_spacing(p, before=6, after=6)
            if alt_text:
                cap = doc.add_paragraph()
                cap.alignment = WD_ALIGN_PARAGRAPH.CENTER
                add_run(cap, alt_text, italic=True, color=s['secondary'], size=9)
                set_spacing(cap, after=6)
        except Exception as e:
            p = doc.add_paragraph()
            msg = '[Image: ' + alt_text + ' - error: ' + str(e) + ']'
            add_run(p, msg, color=s['accent'], size=10, italic=True)
    else:
        p = doc.add_paragraph()
        msg = '[Image: ' + alt_text + ' - not found: ' + img_path + ']'
        add_run(p, msg, color=s['accent'], size=10, italic=True)


def convert(md_content, output_path, args):
    """Convert markdown to a styled Word document."""
    style_name = args.style
    base_size = args.font_size
    base_font = args.font_family
    s = STYLES[style_name]
    doc = Document()
    section = doc.sections[0]
    section.top_margin = Cm(2.54)
    section.bottom_margin = Cm(2.54)
    section.left_margin = Cm(2.54)
    section.right_margin = Cm(2.54)
    headings = []
    for line in md_content.split('\n'):
        m = re.match(r'^(#{1,6})\s+(.+)$', line)
        if m:
            headings.append((len(m.group(1)), m.group(2).strip()))
    if not args.no_cover:
        build_cover_page(doc, args.title, args.author, style_name)
    if not args.no_toc and headings:
        build_toc(doc, headings, style_name)
    lines = md_content.split('\n')
    i = 0
    in_code_block = False
    code_buffer = []
    input_dir = os.path.dirname(os.path.abspath(args.input))
    while i < len(lines):
        line = lines[i]
        stripped = line.strip()
        if stripped.startswith('```'):
            if in_code_block:
                build_code_block(doc, '\n'.join(code_buffer), style_name, base_size - 1)
                code_buffer = []
                in_code_block = False
            else:
                in_code_block = True
                code_buffer = []
            i += 1
            continue
        if in_code_block:
            code_buffer.append(line)
            i += 1
            continue
        if stripped.startswith('|') and stripped.endswith('|'):
            table_data = []
            while i < len(lines) and '|' in lines[i].strip():
                cells = [c.strip() for c in lines[i].strip().split('|')[1:-1]]
                table_data.append(cells)
                i += 1
            if len(table_data) >= 2 and any('---' in c for c in table_data[1]):
                table_data.pop(1)
            if table_data:
                build_table(doc, table_data, style_name, base_size)
            continue
        if re.match(r'^[-*_]{3,}\s*$', stripped):
            p = doc.add_paragraph()
            add_run(p, '\u2501' * 60, color=s['secondary'], size=8)
            set_spacing(p, before=6, after=6)
            i += 1
            continue
        m = re.match(r'^(#{1,6})\s+(.+)$', line)
        if m:
            build_heading(doc, m.group(2).strip(), len(m.group(1)),
                          style_name, base_size)
            i += 1
            continue
        if stripped.startswith('>'):
            quotes = []
            while i < len(lines) and lines[i].strip().startswith('>'):
                quotes.append(lines[i].strip().lstrip('>').strip())
                i += 1
            build_blockquote(doc, ' '.join(quotes), style_name, base_size)
            continue
        if re.match(r'^[\s]*[-*+]\s+', stripped):
            items = []
            while i < len(lines):
                sl = lines[i].strip()
                if re.match(r'^[\s]*[-*+]\s+', sl):
                    items.append(re.sub(r'^[\s]*[-*+]\s+', '', sl))
                    i += 1
                else:
                    break
            build_list(doc, items, False, style_name, base_size)
            continue
        if re.match(r'^\s*\d+[\.)]\s+', stripped):
            items = []
            while i < len(lines):
                sl = lines[i].strip()
                if re.match(r'^\s*\d+[\.)]\s+', sl):
                    items.append(re.sub(r'^\s*\d+[\.)]\s+', '', sl))
                    i += 1
                else:
                    break
            build_list(doc, items, True, style_name, base_size)
            continue
        img_match = re.match(r'!\[(.*?)\]\((.*?)\)', stripped)
        if img_match:
            build_image(doc, img_match.group(2), img_match.group(1),
                        style_name, input_dir)
            i += 1
            continue
        if stripped == '':
            i += 1
            continue
        p = doc.add_paragraph()
        apply_inline(p, stripped, style_name, base_size, base_font)
        set_spacing(p, before=3, after=3)
        p.alignment = WD_ALIGN_PARAGRAPH.LEFT
        i += 1
    doc.save(output_path)
    return True


def main():
    """Entry point."""
    parser = argparse.ArgumentParser(
        description='Convert Markdown to a beautiful Word document.',
        formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument('input', help='Input Markdown file path')
    parser.add_argument('-o', '--output', help='Output Word file path')
    parser.add_argument('--style', choices=['modern', 'classic', 'minimal'],
                        default='modern', help='Visual style')
    parser.add_argument('--title', help='Document title')
    parser.add_argument('--author', default='co-shell', help='Author name')
    parser.add_argument('--font-size', type=int, default=11,
                        help='Base font size in pt')
    parser.add_argument('--font-family', default='Arial', help='Font family')
    parser.add_argument('--no-toc', action='store_true',
                        help='Skip table of contents')
    parser.add_argument('--no-cover', action='store_true',
                        help='Skip cover page')
    parser.add_argument('--debug', action='store_true',
                        help='Print debug info')
    args = parser.parse_args()
    if not os.path.exists(args.input):
        print('Error: File not found: ' + args.input)
        sys.exit(1)
    if not args.output:
        base = os.path.splitext(os.path.basename(args.input))[0]
        args.output = base + '.docx'
    if not args.title:
        args.title = os.path.splitext(os.path.basename(args.input))[0]
    with open(args.input, 'r', encoding='utf-8') as f:
        md_content = f.read()
    if args.debug:
        print('Input:   ' + args.input)
        print('Output:  ' + args.output)
        print('Style:   ' + args.style)
        print('Font:    ' + args.font_family + ' ' + str(args.font_size) + 'pt')
        print('Title:   ' + args.title)
        print('Author:  ' + args.author)
        print('Size:    ' + str(len(md_content)) + ' chars')
        print('-' * 50)
    try:
        convert(md_content, args.output, args)
        size_kb = os.path.getsize(args.output) / 1024.0
        print('Successfully converted to: ' + args.output)
        print('File size: %.1f KB' % size_kb)
    except Exception as e:
        print('Error: ' + str(e))
        if args.debug:
            import traceback
            traceback.print_exc()
        sys.exit(1)


if __name__ == '__main__':
    main()
