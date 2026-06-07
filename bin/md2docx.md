# md2docx — Markdown to Word Document Converter

## Description
Converts Markdown files to beautifully styled Word (.docx) documents.
Supports multiple visual styles including Chinese government document format
(GB/T 9704-2012), modern, classic, and minimal. Handles headings, tables,
code blocks, blockquotes, lists, images, inline formatting, cover pages,
and table of contents.

## Usage
```
python3 bin/md2docx.py input.md -o output.docx
python3 bin/md2docx.py input.md --style modern
python3 bin/md2docx.py input.md --style official
```

## Arguments
| Argument | Required | Default | Description |
|----------|----------|---------|-------------|
| `input`  | Yes      | —       | Input Markdown file path. |
| `-o`/`--output` | No | `{input}.docx` | Output Word file path. |
| `--style` | No | `official` | Visual style: `modern`, `classic`, `minimal`, `official`. |
| `--title` | No | filename | Document title. |
| `--author` | No | `co-shell` | Author name. |
| `--font-size` | No | `16` | Base font size in pt. |
| `--font-family` | No | `仿宋` | Font family. |
| `--no-toc` | No | — | Skip table of contents. |
| `--no-cover` | No | — | Skip cover page. |
| `--debug` | No | — | Print debug info. |

## Dependencies
- Python 3
- python-docx (`pip install python-docx`)

## Example (called by LLM)
1. Complete a research report written in Markdown.
2. Convert to Word for delivery: `python3 bin/md2docx.py report.md -o report.docx --style official --author "User Name"`
3. Present the output file to the user.