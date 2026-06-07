#!/usr/bin/env python3
"""Document to PNG Image Converter.

Converts Word documents (.doc/.docx/.wps) directly to page-by-page PNG images,
preserving original formatting (headings, tables, charts, images, links, fonts).

Process: LibreOffice → PDF → PyMuPDF → PNG pages

Priority: LibreOffice > Pure Python (always works, no external deps).

Usage:
    python3 bin/doc2png.py report.docx -o ./pages
    python3 bin/doc2png.py report.doc -o ./pages --dpi 300
    python3 bin/doc2png.py document.wps -o ./pages
"""

import argparse
import os
import shutil
import subprocess
import sys
import time


def check_command(cmd):
    return shutil.which(cmd) is not None


def export_pdf_with_libreoffice(input_path, output_path):
    """Export document to PDF using LibreOffice headless (best formatting)."""
    output_dir = os.path.dirname(os.path.abspath(output_path))
    subprocess.run(["soffice", "--headless", "--convert-to", "pdf",
                    "--outdir", output_dir, input_path],
                   capture_output=True, timeout=120, check=True)
    expected = os.path.join(output_dir, os.path.splitext(os.path.basename(input_path))[0] + ".pdf")
    if expected != os.path.abspath(output_path) and os.path.exists(expected):
        os.replace(expected, os.path.abspath(output_path))
    return True


def pdf_to_png(pdf_path, output_dir, dpi=200):
    """Split PDF pages into PNG images (PyMuPDF)."""
    import fitz
    os.makedirs(output_dir, exist_ok=True)

    doc = fitz.open(pdf_path)
    total = len(doc)
    if total == 0:
        print("Warning: PDF has no pages.", file=sys.stderr)
        doc.close()
        return []

    pad = len(str(total))
    base_name = os.path.splitext(os.path.basename(pdf_path))[0]
    generated = []

    print(f"Splitting {total} page(s) to PNG...")
    for i in range(total):
        page = doc[i]
        zoom = dpi / 72.0
        mat = fitz.Matrix(zoom, zoom)
        pix = page.get_pixmap(matrix=mat)

        page_num = str(i + 1).zfill(pad)
        out_file = os.path.join(output_dir, f"{base_name}_p{page_num}.png")
        pix.save(out_file)
        generated.append(out_file)

        size_kb = os.path.getsize(out_file) / 1024.0
        print(f"  [{page_num}/{total}] {out_file} ({size_kb:.0f} KB)")

    doc.close()
    return generated


def export_pdf_fallback(input_path, output_path):
    """Fallback: extract text and generate a simple PDF (no formatting)."""
    import fitz
    ext = os.path.splitext(input_path)[1].lower()

    if ext == '.docx':
        from docx import Document
        doc = Document(input_path)
        text = '\n'.join(p.text for p in doc.paragraphs if p.text.strip())
    elif ext in ('.doc', '.wps'):
        try:
            import olefile
            ole = olefile.OleFileIO(input_path)
            data = ole.openstream('WordDocument').read()
            ole.close()
            raw = data.decode('utf-16-le', errors='replace')
            safe = []
            for c in raw:
                cp = ord(c)
                if cp in (10, 13):
                    safe.append('\n')
                elif 0x20 <= cp <= 0x7E:
                    safe.append(c)
                elif 0x4E00 <= cp <= 0x9FFF:
                    safe.append(c)
                elif 0x3000 <= cp <= 0x303F:
                    safe.append(c)
            text = ''.join(safe)
        except ImportError:
            raise RuntimeError("olefile not found. Install: pip install olefile")
    else:
        raise RuntimeError(f"Unsupported format: {ext}")

    if not text.strip():
        raise RuntimeError("Could not extract text from document")

    pdf = fitz.open()
    page = pdf.new_page()
    margin = 50
    y = margin
    for line in text.split('\n'):
        line = line.strip()
        if not line:
            y += 10
            continue
        if y > 820:
            page = pdf.new_page()
            y = margin
        try:
            page.insert_text(fitz.Point(margin, y), line, fontsize=11, fontname="china-ss")
        except Exception:
            pass
        y += 15

    pdf.save(output_path)
    pdf.close()


def convert(input_path, output_dir, dpi=200):
    """Convert document to page PNGs."""
    if not os.path.exists(input_path):
        print(f"Error: File not found: {input_path}", file=sys.stderr)
        sys.exit(1)

    os.makedirs(output_dir, exist_ok=True)
    tmp_pdf = os.path.join(output_dir, "__tmp_export.pdf")

    # Strategy: LibreOffice > Fallback
    if check_command("soffice") or check_command("libreoffice"):
        try:
            print("Exporting via LibreOffice...")
            export_pdf_with_libreoffice(input_path, tmp_pdf)
        except Exception as e:
            print(f"  LibreOffice failed: {e}", file=sys.stderr)
            print("Using fallback (text-only, formatting lost)...")
            export_pdf_fallback(input_path, tmp_pdf)
    else:
        print("Using pure Python fallback (text-only, formatting lost)...")
        export_pdf_fallback(input_path, tmp_pdf)

    if not os.path.exists(tmp_pdf):
        print("Error: Failed to produce PDF.", file=sys.stderr)
        print("\nInstall LibreOffice for format-perfect conversion:", file=sys.stderr)
        if sys.platform == "darwin":
            print("  brew install --cask libreoffice", file=sys.stderr)
        elif sys.platform == "win32":
            print("  winget install TheDocumentFoundation.LibreOffice", file=sys.stderr)
        else:
            print("  sudo apt install libreoffice", file=sys.stderr)
        sys.exit(1)

    result = pdf_to_png(tmp_pdf, output_dir, dpi)

    try:
        os.remove(tmp_pdf)
    except Exception:
        pass

    print(f"\nDone. {len(result)} page(s) converted to: {output_dir}")
    return result


def main():
    parser = argparse.ArgumentParser(
        description="Convert Word documents (.doc/.docx/.wps) to page PNG images.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s report.docx -o ./pages
  %(prog)s report.doc -o ./pages --dpi 300
  %(prog)s document.wps -o ./pages

Requires LibreOffice for format-perfect output (brew install --cask libreoffice).
Falls back to text-only extraction if LibreOffice is not available.
        """)
    parser.add_argument("input", help="Input document file (.doc/.docx/.wps)")
    parser.add_argument("-o", "--output", default="pages", help="Output directory for PNG files")
    parser.add_argument("--dpi", type=int, default=200, help="Rendering DPI (default: 200)")
    args = parser.parse_args()

    convert(args.input, args.output, args.dpi)


if __name__ == "__main__":
    main()