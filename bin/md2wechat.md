# md2wechat — Markdown to WeChat Official Account Formatter

## Description
Converts Markdown files into HTML suitable for pasting into the WeChat
Official Account editor. Handles headings, tables, code blocks, blockquotes,
bold, italic, links, lists, and horizontal rules with inline CSS styling
that is compatible with WeChat's editor.

## Usage
```
python3 bin/md2wechat.py input.md output.html
python3 bin/md2wechat.py input.md   # prints to stdout
```

## Arguments
| Argument | Required | Default | Description |
|----------|----------|---------|-------------|
| `input`  | Yes      | —       | Input Markdown file path. |
| `output` | No       | stdout  | Output HTML file path (omit for stdout). |

## Dependencies
- Python 3 (standard library only, no extra dependencies)

## Example (called by LLM)
1. Write an article in Markdown format for a WeChat Official Account.
2. Convert to WeChat-compatible HTML: `python3 bin/md2wechat.py article.md article.html`
3. Open the HTML file and paste the content into the WeChat editor.