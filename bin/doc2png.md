# doc2png — Document to PNG Image Converter

## Description
Converts Word documents (.doc/.docx/.wps) directly to page-by-page PNG images,
preserving original formatting (headings, tables, charts, images, links, fonts).

Process: LibreOffice → PDF → PyMuPDF → PNG pages

## Usage
```
python3 bin/doc2png.py report.docx -o ./pages
python3 bin/doc2png.py report.doc -o ./pages --dpi 300
python3 bin/doc2png.py document.wps -o ./pages
```

## Arguments
| Argument | Required | Default | Description |
|----------|----------|---------|-------------|
| `input`  | Yes      | —       | Input document file (.doc/.docx/.wps) |
| `-o`/`--output` | No | `pages` | Output directory for PNG files |
| `--dpi`  | No       | `200`   | Rendering DPI (higher = better quality but larger files) |

## Dependencies
- Python 3
- PyMuPDF (`pip install PyMuPDF`)
- **LibreOffice** (recommended, for format-perfect export):
  - macOS: `brew install --cask libreoffice`
  - Windows: `winget install TheDocumentFoundation.LibreOffice`
  - Linux: `sudo apt install libreoffice`
- If LibreOffice is not installed, falls back to text-only extraction (formatting lost).

## Example Workflow (called by LLM)
1. User has a .docx document that needs visual analysis of complex tables/charts.
2. Convert to page PNGs: `python3 bin/doc2png.py report.docx -o ./report_pages`
3. Use `add_images` tool to load the PNGs into image context.
4. Ask the vision model to analyze the content of each page image.