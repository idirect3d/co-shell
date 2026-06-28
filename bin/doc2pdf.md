# doc2pdf — Document to PDF Converter

## Description
Converts Word documents (.doc/.docx/.wps) to PDF using LibreOffice (best formatting)
or pure Python fallback (text extraction, no external dependencies).

## Usage
```
python3 bin/doc2pdf.py report.docx -o report.pdf
python3 bin/doc2pdf.py report.doc -o report.pdf
python3 bin/doc2pdf.py document.wps -o document.pdf
python3 bin/doc2pdf.py input.docx -o output.pdf --engine python
```

## Arguments
| Argument | Required | Default | Description |
|----------|----------|---------|-------------|
| `input`  | Yes      | —       | Input document file (.doc/.docx/.wps) |
| `-o`/`--output` | Yes | — | Output PDF file path |
| `--engine` | No | `auto` | Engine: `auto` (LibreOffice first), `libreoffice`, `python` |

## Engines
| Engine | Description |
|--------|-------------|
| `libreoffice` | `soffice --headless` (recommended, perfect formatting) |
| `python` | Pure Python text extraction (always works, no external deps) |

## Dependencies
- Python 3
- PyMuPDF (`pip install PyMuPDF`)
- **LibreOffice** (recommended):
  - macOS: `brew install --cask libreoffice`
  - Windows: `winget install TheDocumentFoundation.LibreOffice`
  - Linux: `sudo apt install libreoffice`
- For Python engine: `python-docx` (for .docx), `olefile` (for .doc/.wps)

## Example Workflow (called by LLM)
1. User needs to convert a Word document for PDF viewing or PNG conversion.
2. Convert: `python3 bin/doc2pdf.py report.docx -o report.pdf`
3. If PNG pages needed: `python3 bin/pdf2png.py report.pdf -o ./pages`
4. Use `visual_analysis` to load page images and analyze with vision model.