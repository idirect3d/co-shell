# wps2pdf — WPS (.wps) to PDF Converter

## Description
Converts .wps (WPS Office Writer format) files to PDF. Supports printing,
document archiving, and further conversion to PNG (via pdf2png) for
multimodal LLM visual recognition of WPS document content.

## Usage
```
python3 bin/wps2pdf.py input.wps -o output.pdf
python3 bin/wps2pdf.py input.wps -o output.pdf --engine wps
```

## Arguments
| Argument | Required | Default | Description |
|----------|----------|---------|-------------|
| `input`  | Yes      | —       | Input .wps file path. |
| `-o`/`--output` | Yes | — | Output PDF file path. |
| `--engine` | No | `auto` | Conversion engine: `auto` (WPS first), `wps`, `libreoffice`. |

## Supported Engines
| Engine | Requirement |
|--------|-------------|
| `wps` (default) | WPS Office (`wps2pdf` or `wps` on PATH) |
| `libreoffice` | LibreOffice (`soffice` on PATH) |

## Dependencies
- Python 3 (no extra packages required)
- WPS Office or LibreOffice

## Example Workflow (called by LLM)
1. User has a .wps document that needs content analysis.
2. Convert to PDF: `python3 bin/wps2pdf.py document.wps -o document.pdf`
3. Split PDF to PNG pages: `python3 bin/pdf2png.py document.pdf -o ./pages`
4. Use `add_images` to load page images and analyze with vision model.