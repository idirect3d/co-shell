# doc2pdf — Old-Format DOC to PDF Converter

## Description
Converts old-format .doc (Word 97-2003) files to PDF. Used for printing, document
archiving, and further conversion to PNG (via pdf2png) for multimodal LLM
visual recognition of legacy document content.

## Usage
```
python3 bin/doc2pdf.py input.doc -o output.pdf
```

## Arguments
| Argument | Required | Default | Description |
|----------|----------|---------|-------------|
| `input`  | Yes      | —       | Input .doc file path. |
| `-o`/`--output` | Yes | — | Output PDF file path. |

## Supported Engines (auto-detected in order)
| Engine | Platform | Requirement |
|--------|----------|-------------|
| `textutil` | macOS | Built-in, no install needed |
| `LibreOffice` | All | `soffice` on PATH |
| `WPS Office` | All (Linux best) | `wps2pdf` or `wps` on PATH |

## Dependencies
- Python 3 (no extra packages required)
- One of: macOS textutil (built-in), LibreOffice, or WPS Office

If none of the above is installed, the tool prints installation instructions
(recommends WPS for best .doc compatibility).

## Example Workflow (called by LLM)
1. User has a legacy .doc file that needs content analysis.
2. Convert to PDF: `python3 bin/doc2pdf.py old_report.doc -o report.pdf`
3. Split PDF to PNG pages: `python3 bin/pdf2png.py report.pdf -o ./pages`
4. Use `add_images` to load page images and analyze with vision model.