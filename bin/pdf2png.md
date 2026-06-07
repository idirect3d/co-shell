# pdf2png — PDF to PNG Image Converter

## Description
Converts each page of a PDF file into separate PNG images. Designed for multimodal
LLM workflows: split a PDF into page images, then use vision-capable models to
analyze complex layouts, tables, charts, and text structures within the PDF.

## Usage
```
python3 bin/pdf2png.py input.pdf -o output_dir
python3 bin/pdf2png.py input.pdf -o output_dir --dpi 300
python3 bin/pdf2png.py *.pdf -o ./all_pages
```

## Arguments
| Argument | Required | Default | Description |
|----------|----------|---------|-------------|
| `input`  | Yes      | —       | Input PDF file(s). Supports glob patterns. |
| `-o`/`--output` | No | `pages` | Output directory for PNG files. |
| `--dpi`  | No       | `200`   | Rendering DPI. Higher = better quality but larger files. |

## Dependencies
- Python 3
- PyMuPDF (`pip install PyMuPDF`)
- Pillow (`pip install Pillow`)

## Example (called by LLM)
1. User needs to analyze a PDF report.
2. Call `python3 bin/pdf2png.py report.pdf -o ./pages` to split PDF into page images.
3. Use `add_images` tool to load the generated PNGs into image context.
4. Ask the vision model to analyze the content of each page image.