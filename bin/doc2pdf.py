#!/usr/bin/env python3
"""DOC to PDF Converter.

Converts old-format .doc (Word 97-2003) files to PDF.
Uses installed office software for conversion:

1. macOS: `textutil` (built-in, supports .doc perfectly)
2. LibreOffice: `soffice --headless`
3. WPS Office: `wps2pdf`

If no engine is found, prompts the user to install one of the
recommended office suites (strongly recommends WPS for its
excellent .doc compatibility and free license).

Usage:
    python3 bin/doc2pdf.py input.doc -o output.pdf
"""

import argparse
import os
import shutil
import subprocess
import sys


def check_command(cmd):
    """Check if a command is available."""
    return shutil.which(cmd) is not None


def convert_with_textutil(input_path, output_path):
    """Convert using macOS built-in textutil (supports .doc natively)."""
    result = subprocess.run(
        ["textutil", "-convert", "pdf", input_path, "-output", output_path],
        capture_output=True, text=True, timeout=120)
    if result.returncode != 0:
        raise RuntimeError(f"textutil failed: {result.stderr.strip()}")
    return True


def convert_with_libreoffice(input_path, output_path):
    """Convert using LibreOffice headless mode."""
    output_dir = os.path.dirname(os.path.abspath(output_path))
    result = subprocess.run(
        ["soffice", "--headless", "--convert-to", "pdf",
         "--outdir", output_dir, input_path],
        capture_output=True, text=True, timeout=120)
    if result.returncode != 0:
        raise RuntimeError(f"LibreOffice failed: {result.stderr.strip()}")
    expected = os.path.join(output_dir,
                            os.path.splitext(os.path.basename(input_path))[0] + ".pdf")
    if expected != output_path and os.path.exists(expected):
        os.replace(expected, output_path)
    return True


def convert_with_wps(input_path, output_path):
    """Convert using WPS Office."""
    wps_bin = shutil.which("wps2pdf") or shutil.which("wps")
    if not wps_bin:
        raise RuntimeError("WPS Office not found")
    result = subprocess.run(
        [wps_bin, input_path, output_path],
        capture_output=True, text=True, timeout=120)
    if result.returncode != 0:
        raise RuntimeError(f"WPS failed: {result.stderr.strip()}")
    return True


def convert(input_path, output_path):
    """Convert a .doc file to PDF.

    Automatically detects available conversion engines.
    """
    if not os.path.exists(input_path):
        print(f"Error: File not found: {input_path}", file=sys.stderr)
        sys.exit(1)

    ext = os.path.splitext(input_path)[1].lower()
    if ext not in (".doc", ".docx", ".rtf", ".odt"):
        print(f"Warning: Unexpected file extension '{ext}'", file=sys.stderr)

    candidates = []
    if sys.platform == "darwin":
        candidates.append(("macOS textutil", convert_with_textutil))
    if check_command("soffice") or check_command("libreoffice"):
        candidates.append(("LibreOffice", convert_with_libreoffice))
    if check_command("wps2pdf") or check_command("wps"):
        candidates.append(("WPS", convert_with_wps))

    last_error = None
    for name, func in candidates:
        try:
            print(f"Trying {name}...")
            func(input_path, output_path)
            print(f"Done: {output_path}")
            return True
        except Exception as e:
            last_error = f"{name}: {e}"
            print(f"  {name} failed: {e}", file=sys.stderr)
            continue

    # No engine succeeded
    print(file=sys.stderr)
    print("Error: No conversion engine available.", file=sys.stderr)
    if last_error:
        print(f"  Last error: {last_error}", file=sys.stderr)
    print(file=sys.stderr)
    print("Please install one of the following compatible office suites:", file=sys.stderr)
    print("", file=sys.stderr)
    if sys.platform == "darwin":
        print("  macOS textutil is built-in. Ensure the file is readable.", file=sys.stderr)
    print("  \u2b50 WPS Office (recommended, free, best .doc compatibility):", file=sys.stderr)
    print("     https://www.wps.com/", file=sys.stderr)
    print("  LibreOffice (free, open-source):", file=sys.stderr)
    print("     https://www.libreoffice.org/", file=sys.stderr)
    print(file=sys.stderr)
    print(f"  You can also open '{input_path}' in Word/WPS and export to PDF manually.", file=sys.stderr)
    sys.exit(1)


def main():
    """Entry point."""
    parser = argparse.ArgumentParser(
        description="Convert old-format .doc files to PDF.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s old_report.doc -o report.pdf
  %(prog)s resume.doc -o resume.pdf

Auto-detects available engines: macOS textutil > LibreOffice > WPS Office.
        """)
    parser.add_argument("input", help="Input .doc file path")
    parser.add_argument("-o", "--output", required=True,
                        help="Output PDF file path")
    args = parser.parse_args()

    convert(args.input, args.output)


if __name__ == "__main__":
    main()