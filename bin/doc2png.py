#!/usr/bin/env python3
"""Document to PNG Image Converter.

Converts Word documents (.doc/.docx/.wps) directly to page-by-page PNG images,
preserving original formatting (headings, tables, charts, images, links, fonts).

Process:
1. Use WPS Office or LibreOffice to export to PDF (preserving full formatting)
2. Split PDF pages into individual PNG images (using same engine as pdf2png)

If no office suite is available, falls back to text extraction + basic rendering.

Usage:
    python3 bin/doc2png.py input.docx -o ./pages
    python3 bin/doc2png.py input.doc -o ./pages --dpi 300
"""

import argparse
import os
import shutil
import subprocess
import sys
import tempfile


def check_command(cmd):
    return shutil.which(cmd) is not None


def find_wps_macos():
    """Find WPS Office executable on macOS by bundle path."""
    candidates = [
        "/Applications/wpsoffice.app/Contents/MacOS/wpsoffice",
        "/Applications/WPS Office.app/Contents/MacOS/wps",
        os.path.expanduser("~/Applications/WPS Office.app/Contents/MacOS/wps"),
    ]
    for p in candidates:
        if os.path.isfile(p) and os.access(p, os.X_OK):
            return p
    # Try mdfind as fallback
    try:
        r = subprocess.run(["mdfind", "kMDItemCFBundleIdentifier == 'com.kingsoft.wpsoffice*'"],
                           capture_output=True, text=True, timeout=5)
        for line in r.stdout.split('\n'):
            line = line.strip()
            if not line:
                continue
            exe = os.path.join(line, "Contents", "MacOS", "wpsoffice")
            if os.path.isfile(exe) and os.access(exe, os.X_OK):
                return exe
    except Exception:
        pass
    return None


def find_wps_windows():
    import glob
    for pattern in [
        r"C:\Program Files\WPS Office\*\wps.exe",
        r"C:\Program Files (x86)\WPS Office\*\wps.exe",
    ]:
        matches = glob.glob(pattern)
        if matches:
            return matches[0]
    return None


def export_pdf_with_wps(input_path, output_path):
    """Export document to PDF using WPS Office (preserves full formatting)."""
    if sys.platform == "darwin" and find_wps_macos():
        # macOS: use AppleScript print-to-PDF via UI automation
        abs_in = os.path.abspath(input_path)
        abs_out = os.path.abspath(output_path)

        # Close any existing doc first, then open new one and print to PDF
        script = (
            f'tell application "wpsoffice"\n'
            f'    activate\n'
            f'    open POSIX file "{abs_in}"\n'
            f'    delay 5\n'
            f'end tell\n'
            f'delay 2\n'
            f'tell application "System Events"\n'
            f'    tell process "wpsoffice"\n'
            f'        keystroke "p" using {{command down}}\n'
            f'        delay 3\n'
            f'        keystroke return\n'
            f'        delay 1\n'
            f'    end tell\n'
            f'end tell\n'
            f'tell application "wpsoffice" to quit'
        )
        subprocess.run(["osascript", "-e", script], capture_output=True, text=True, timeout=120)
        # Check the default save location (usually ~/Desktop or ~/Documents)
        home = os.path.expanduser("~")
        for check_dir in [home + "/Desktop", home + "/Documents", "/tmp"]:
            for f in os.listdir(check_dir):
                if f.lower().endswith(".pdf") and not f.startswith("."):
                    candidate = os.path.join(check_dir, f)
                    mtime = os.path.getmtime(candidate)
                    if time.time() - mtime < 30:  # just created
                        os.replace(candidate, abs_out)
                        return True
        raise RuntimeError("WPS print-to-PDF failed: no PDF found")

    elif sys.platform == "linux" and check_command("wps2pdf"):
        subprocess.run(["wps2pdf", input_path, output_path], capture_output=True, timeout=120, check=True)
        return True

    elif sys.platform == "win32" and find_wps_windows():
        try:
            import win32com.client
            wps_app = win32com.client.Dispatch("KWps.Application")
            wps_app.Visible = False
            doc = wps_app.Documents.Open(os.path.abspath(input_path))
            doc.ExportAsFixedFormat(os.path.abspath(output_path), 17)
            doc.Close()
            wps_app.Quit()
            return True
        except ImportError:
            raise RuntimeError("pywin32 not available for WPS COM automation")

    raise RuntimeError("WPS Office not found or export failed")


def export_pdf_with_libreoffice(input_path, output_path):
    """Export document to PDF using LibreOffice headless."""
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
    """Fallback: extract text and generate a simple PDF."""
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

    # Create simple PDF from text
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

    # Strategy: WPS > LibreOffice > Fallback
    success = False
    errors = []

    # 1. Try WPS
    wps_path = None
    if sys.platform == "darwin":
        wps_path = find_wps_macos()
    elif sys.platform == "linux":
        wps_path = "wps2pdf" if check_command("wps2pdf") else None
    elif sys.platform == "win32":
        wps_path = find_wps_windows()

    if wps_path:
        try:
            print("Exporting via WPS Office...")
            export_pdf_with_wps(input_path, tmp_pdf)
            success = True
        except Exception as e:
            errors.append(f"WPS: {e}")
            print(f"  WPS failed: {e}", file=sys.stderr)

    # 2. Try LibreOffice
    if not success and (check_command("soffice") or check_command("libreoffice")):
        try:
            print("Exporting via LibreOffice...")
            export_pdf_with_libreoffice(input_path, tmp_pdf)
            success = True
        except Exception as e:
            errors.append(f"LibreOffice: {e}")
            print(f"  LibreOffice failed: {e}", file=sys.stderr)

    # 3. Fallback: pure Python
    if not success:
        print("Using pure Python fallback (text-only, formatting lost)...")
        try:
            export_pdf_fallback(input_path, tmp_pdf)
            success = True
        except Exception as e:
            errors.append(f"Fallback: {e}")
            print(f"  Fallback failed: {e}", file=sys.stderr)

    if not success or not os.path.exists(tmp_pdf):
        print("Error: All conversion methods failed.", file=sys.stderr)
        for e in errors:
            print(f"  {e}", file=sys.stderr)
        print("\nPlease install WPS Office (recommended):", file=sys.stderr)
        print("  macOS: brew install --cask wpsoffice", file=sys.stderr)
        print("  Windows: winget install Kingsoft.WPSOffice", file=sys.stderr)
        print("  Linux: wget https://wps.com/linux/wps.deb && sudo dpkg -i wps.deb", file=sys.stderr)
        sys.exit(1)

    # Split PDF to PNG
    result = pdf_to_png(tmp_pdf, output_dir, dpi)

    # Clean up temp PDF
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

Formats are preserved when using WPS Office or LibreOffice.
Fallback extracts text only (no formatting).
        """)
    parser.add_argument("input", help="Input document file (.doc/.docx/.wps)")
    parser.add_argument("-o", "--output", default="pages", help="Output directory for PNG files")
    parser.add_argument("--dpi", type=int, default=200, help="Rendering DPI (default: 200)")
    args = parser.parse_args()

    convert(args.input, args.output, args.dpi)


if __name__ == "__main__":
    main()