#!/usr/bin/env python3
"""DOC to PDF Converter.

Converts old-format .doc (Word 97-2003) files to PDF with pure Python fallback.
Also attempts to use WPS Office or LibreOffice if available for better formatting.

Priority: WPS Office > LibreOffice > Pure Python (always works, never depends on external apps).

Usage:
    python3 bin/doc2pdf.py input.doc -o output.pdf
"""

import argparse
import os
import shutil
import subprocess
import sys


def check_command(cmd):
    return shutil.which(cmd) is not None


def find_wps_macos():
    import subprocess
    try:
        r = subprocess.run(["mdfind", "kMDItemCFBundleIdentifier == 'com.kingsoft.wpsoffice*'"],
                           capture_output=True, text=True, timeout=5)
        for p in r.stdout.split('\n'):
            p = p.strip()
            if not p:
                continue
            exe = os.path.join(p, "Contents", "MacOS", "wps")
            if os.path.isfile(exe) and os.access(exe, os.X_OK):
                return exe
    except Exception:
        pass
    for p in [
        "/Applications/WPS Office.app/Contents/MacOS/wps",
        os.path.expanduser("~/Applications/WPS Office.app/Contents/MacOS/wps"),
    ]:
        if os.path.isfile(p) and os.access(p, os.X_OK):
            return p
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


def convert_with_wps(input_path, output_path):
    wps2pdf = shutil.which("wps2pdf")
    if wps2pdf:
        subprocess.run([wps2pdf, input_path, output_path], capture_output=True, timeout=120, check=True)
        return
    if sys.platform == "darwin":
        wps = find_wps_macos()
        if wps:
            subprocess.run([wps, os.path.abspath(input_path), '--export-to-pdf', os.path.abspath(output_path)],
                           capture_output=True, timeout=120, shell=True)
            return
    if sys.platform == "win32":
        wps = find_wps_windows()
        if wps:
            try:
                import win32com.client
                app = win32com.client.Dispatch("KWps.Application")
                app.Visible = False
                doc = app.Documents.Open(os.path.abspath(input_path))
                doc.ExportAsFixedFormat(os.path.abspath(output_path), 17)
                doc.Close()
                app.Quit()
                return
            except ImportError:
                pass
    raise RuntimeError("WPS Office not found or conversion failed")


def convert_with_libreoffice(input_path, output_path):
    output_dir = os.path.dirname(os.path.abspath(output_path))
    subprocess.run(["soffice", "--headless", "--convert-to", "pdf", "--outdir", output_dir, input_path],
                   capture_output=True, timeout=120, check=True)
    expected = os.path.join(output_dir, os.path.splitext(os.path.basename(input_path))[0] + ".pdf")
    if expected != os.path.abspath(output_path) and os.path.exists(expected):
        os.replace(expected, os.path.abspath(output_path))


def convert_with_python(input_path, output_path):
    """Pure Python conversion for .doc files: extract text via olefile -> PyMuPDF.

    Old .doc files are OLE2 compound documents. We extract text from the
    WordDocument binary stream via UTF-16LE decoding, then generate a PDF
    with the PyMuPDF built-in CJK font (china-ss).
    """
    import fitz

    try:
        import olefile
    except ImportError:
        raise RuntimeError("olefile not found. Install: pip install olefile")

    try:
        ole = olefile.OleFileIO(input_path)
        data = ole.openstream('WordDocument').read()
        ole.close()
    except Exception as e:
        raise RuntimeError(f"Failed to read .doc file: {e}")

    # Decode as UTF-16LE, then keep only characters that china-ss can render
    raw = data.decode('utf-16-le', errors='replace')

    # Build clean text: keep only safe codepoints
    safe_chars = []
    for c in raw:
        cp = ord(c)
        if cp == 0:
            continue
        # Keep newlines
        if cp in (10, 13):
            safe_chars.append('\n')
            continue
        # Skip all control chars except newline
        if cp < 32:
            continue
        # ASCII printable
        if 0x20 <= cp <= 0x7E:
            safe_chars.append(c)
            continue
        # CJK Unified (the main Chinese character block)
        if 0x4E00 <= cp <= 0x9FFF:
            safe_chars.append(c)
            continue
        # CJK Extension A
        if 0x3400 <= cp <= 0x4DBF:
            safe_chars.append(c)
            continue
        # CJK Symbols and Punctuation
        if 0x3000 <= cp <= 0x303F:
            safe_chars.append(c)
            continue
        # Fullwidth forms
        if 0xFF00 <= cp <= 0xFFEF:
            safe_chars.append(c)
            continue
        # Common punctuation (em-dash, en-dash, ellipsis, smart quotes)
        if cp in (0x2013, 0x2014, 0x2018, 0x2019, 0x201C, 0x201D, 0x2026):
            safe_chars.append(c)
            continue

    clean_text = ''.join(safe_chars)

    # Extract meaningful lines (must contain CJK or enough ASCII letters)
    lines = []
    for line in clean_text.split('\n'):
        line = line.strip()
        if not line:
            continue
        cjk = sum(1 for c in line if 0x4E00 <= ord(c) <= 0x9FFF)
        alpha = sum(1 for c in line if c.isascii() and c.isalpha())
        if cjk == 0 and alpha < 3:
            continue
        lines.append(line)

    if not lines:
        raise RuntimeError("Could not extract readable text from .doc file")

    # Generate PDF: render each line
    pdf = fitz.open()
    page = pdf.new_page()
    margin = 50
    y = margin
    font_size = 11

    for line in lines:
        if y > 820:
            page = pdf.new_page()
            y = margin

        # Render line char by char - robustly handles all characters
        x = margin
        for ch in line:
            # Skip control chars and surrogates
            cp = ord(ch)
            if cp < 32 or 0xD800 <= cp <= 0xDFFF:
                continue
            try:
                page.insert_text(fitz.Point(x, y), ch, fontsize=font_size, fontname="china-ss")
                x += 8
            except:
                pass  # skip unrenderable chars silently
        y += 15

    pdf.save(output_path)
    pdf.close()


def convert(input_path, output_path, engine="auto"):
    if not os.path.exists(input_path):
        print(f"Error: File not found: {input_path}", file=sys.stderr)
        sys.exit(1)

    if engine == "auto":
        # 1. Try WPS
        wps_ok = False
        if sys.platform == "linux" and check_command("wps2pdf"):
            wps_ok = True
        elif sys.platform == "darwin" and find_wps_macos():
            wps_ok = True
        elif sys.platform == "win32" and find_wps_windows():
            wps_ok = True

        if wps_ok:
            try:
                print("Trying WPS Office...")
                convert_with_wps(input_path, output_path)
                print(f"Done: {output_path}")
                return
            except Exception as e:
                print(f"  WPS failed: {e}", file=sys.stderr)

        # 2. Try LibreOffice
        if check_command("soffice") or check_command("libreoffice"):
            try:
                print("Trying LibreOffice...")
                convert_with_libreoffice(input_path, output_path)
                print(f"Done: {output_path}")
                return
            except Exception as e:
                print(f"  LibreOffice failed: {e}", file=sys.stderr)

        # 3. Pure Python (always works)
        print("Using pure Python conversion...")
        convert_with_python(input_path, output_path)
        print(f"Done: {output_path}")

    elif engine == "wps":
        convert_with_wps(input_path, output_path)
        print(f"Done: {output_path}")
    elif engine == "libreoffice":
        convert_with_libreoffice(input_path, output_path)
        print(f"Done: {output_path}")
    elif engine == "python":
        convert_with_python(input_path, output_path)
        print(f"Done: {output_path}")
    else:
        print(f"Error: Unknown engine '{engine}'", file=sys.stderr)
        sys.exit(1)


def main():
    parser = argparse.ArgumentParser(
        description="Convert old-format .doc files to PDF.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s old_report.doc -o report.pdf
  %(prog)s report.doc -o report.pdf --engine python

Engines (in order for auto mode):
  wps         - WPS Office (if installed)
  libreoffice - LibreOffice (if installed)
  python      - Pure Python (always works, text extraction)
        """)
    parser.add_argument("input", help="Input .doc file path")
    parser.add_argument("-o", "--output", required=True, help="Output PDF file path")
    parser.add_argument("--engine", choices=["auto", "wps", "libreoffice", "python"],
                        default="auto", help="Conversion engine (default: auto-detect)")
    args = parser.parse_args()

    convert(args.input, args.output, args.engine)


if __name__ == "__main__":
    main()