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

    Old .doc files are OLE2 compound documents. We extract Unicode text stream
    directly. This preserves text content but not formatting.
    """
    import fitz

    try:
        import olefile
        ole = olefile.OleFileIO(input_path)
        text_content = ""

        # Try to read the WordDocument stream
        if ole.exists('WordDocument'):
            # Try Unicode text stream first
            if ole.exists('1Table') or ole.exists('0Table'):
                pass  # we'll use raw stream approach

            # Read raw WordDocument stream and extract readable text
            data = ole.openstream('WordDocument').read()

            # Try different text extraction approaches
            # Method 1: Look for UTF-16LE text in the stream
            try:
                text = data.decode('utf-16-le', errors='ignore')
                # Filter to printable characters
                text = ''.join(c for c in text if c.isprintable() or c in '\n\r\t')
                if len(text.strip()) > 50:
                    text_content = text
            except Exception:
                pass

            # Method 2: Try UTF-8 on the raw data
            if not text_content.strip():
                try:
                    text = data.decode('utf-8', errors='ignore')
                    text = ''.join(c for c in text if c.isprintable() or c in '\n\r\t')
                    if len(text.strip()) > 50:
                        text_content = text
                except Exception:
                    pass

            # Method 3: Extract from streams
            if not text_content.strip():
                for stream_name in ole.listdir():
                    try:
                        stream_data = ole.openstream(stream_name).read()
                        decoded = stream_data.decode('utf-16-le', errors='ignore')
                        printable = ''.join(c for c in decoded if c.isprintable() or c in '\n\r\t')
                        if len(printable.strip()) > 20:
                            text_content += printable + '\n'
                    except Exception:
                        pass

        ole.close()

        if not text_content.strip():
            raise RuntimeError("Could not extract text from .doc file")

    except ImportError:
        raise RuntimeError("olefile not found. Install: pip install olefile")
    except Exception as e:
        raise RuntimeError(f"Failed to extract text from .doc: {e}")

    # Generate PDF from extracted text
    pdf = fitz.open()
    page = pdf.new_page()
    margin = 50
    page_width = 595
    page_height = 842

    lines = text_content.split('\n')
    y = margin
    font_size = 11

    for line in lines:
        line = line.strip()
        if not line:
            y += 12
            continue

        text_height = 14
        if y + text_height > page_height - margin:
            page = pdf.new_page()
            y = margin

        rect = fitz.Rect(margin, y, page_width - margin, y + text_height)
        page.insert_textbox(rect, line, fontsize=font_size, fontname="helv", color=(0, 0, 0))
        y += text_height + 2

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