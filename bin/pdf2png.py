#!/usr/bin/env python3
"""PDF to PNG Converter.

Converts each page of a PDF file into separate PNG images.
Designed to support multimodal LLM PDF content parsing — the LLM can
use this tool to split a PDF into page images, then use image tools
to analyze them with vision-capable models.

Usage:
    python3 pdf2png.py input.pdf -o output_dir
    python3 pdf2png.py input.pdf -o output_dir --dpi 300

Dependencies:
    - PyMuPDF (fitz)
    - Pillow (PIL)
"""

import argparse
import os
import sys

try:
    import fitz
except ImportError:
    print("Error: PyMuPDF (fitz) is required. Install with: pip install PyMuPDF", file=sys.stderr)
    sys.exit(1)

try:
    from PIL import Image
except ImportError:
    print("Error: Pillow (PIL) is required. Install with: pip install Pillow", file=sys.stderr)
    sys.exit(1)


def convert(pdf_path, output_dir, dpi=200):
    """Convert each page of a PDF to a PNG file.

    Args:
        pdf_path: Path to the input PDF file.
        output_dir: Directory to write PNG files to.
        dpi: Resolution for rendering (default: 200).

    Returns:
        List of generated PNG file paths.
    """
    if not os.path.exists(pdf_path):
        print(f"Error: PDF file not found: {pdf_path}", file=sys.stderr)
        sys.exit(1)

    os.makedirs(output_dir, exist_ok=True)

    doc = fitz.open(pdf_path)
    total = len(doc)
    generated = []

    if total == 0:
        print("Warning: PDF has no pages.", file=sys.stderr)
        doc.close()
        return generated

    # Calculate page label padding
    pad = len(str(total))
    base_name = os.path.splitext(os.path.basename(pdf_path))[0]

    print(f"Converting {total} pages from: {pdf_path}")

    for i in range(total):
        page = doc[i]
        # Render at specified DPI
        zoom = dpi / 72.0
        mat = fitz.Matrix(zoom, zoom)
        pix = page.get_pixmap(matrix=mat)

        # Save as PNG
        page_num = str(i + 1).zfill(pad)
        out_file = os.path.join(output_dir, f"{base_name}_p{page_num}.png")
        pix.save(out_file)
        generated.append(out_file)

        size_kb = os.path.getsize(out_file) / 1024.0
        print(f"  [{page_num}/{total}] {out_file} ({size_kb:.0f} KB)")

    doc.close()
    print(f"\nDone. {len(generated)} page(s) converted to: {output_dir}")
    return generated


def main():
    """Entry point."""
    parser = argparse.ArgumentParser(
        description="Convert PDF pages to PNG images for multimodal LLM analysis.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s report.pdf -o ./pages
  %(prog)s report.pdf -o ./pages --dpi 300
  %(prog)s *.pdf -o ./all_pages
        """)
    parser.add_argument("input", nargs="+", help="Input PDF file(s)")
    parser.add_argument("-o", "--output", default="pages",
                        help="Output directory (default: pages)")
    parser.add_argument("--dpi", type=int, default=200,
                        help="Rendering DPI (default: 200, higher = better quality but larger files)")
    args = parser.parse_args()

    for pdf_path in args.input:
        convert(pdf_path, args.output, args.dpi)


if __name__ == "__main__":
    main()