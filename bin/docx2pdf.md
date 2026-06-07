# docx2pdf — DOCX to PDF Converter

## Description
Converts .docx (Word Open XML) files to PDF format. Supports printing, document
sharing, and further conversion to PNG (via pdf2png) for multimodal LLM visual
recognition of document content.

## Usage
```
python3 bin/docx2pdf.py input.docx -o output.pdf
python3 bin/docx2pdf.py input.docx -o output.pdf --engine textutil
```

## Arguments
| Argument | Required | Default | Description |
|----------|----------|---------|-------------|
| `input`  | Yes      | —       | Input .docx file path. |
| `-o`/`--output` | Yes | — | Output PDF file path. |
| `--engine` | No | `auto` | Conversion engine: `auto`, `textutil`, `libreoffice`, `wps`. |

## Supported Engines
| Engine | Platform | Requirement |
|--------|----------|-------------|
| `textutil` | macOS | Built-in, no install needed |
| `libreoffice` | All | `soffice` on PATH |
| `wps` | All (Linux best) | `wps2pdf` or `wps` on PATH |

## Dependencies
- Python 3 (no extra packages required)
- One of: macOS textutil (built-in), LibreOffice, or WPS Office

## Example Workflow (called by LLM)
1. User has a .docx document that needs visual analysis.
2. Convert to PDF: `python3 bin/docx2pdf.py document.docx -o document.pdf`
3. Split PDF to PNG pages: `python3 bin/pdf2png.py document.pdf -o ./pages`
4. Use `add_images` to load page images and analyze with vision model.