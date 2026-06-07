#!/usr/bin/env python3
"""DOC to PDF Converter.

Converts old-format .doc (Word 97-2003) files to PDF.
Uses installed office software for conversion:

1. LibreOffice: `soffice --headless`
2. WPS Office: `wps2pdf`

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
    wps2pdf = shutil.which("wps2pdf")
    if wps2pdf:
        result = subprocess.run(
            [wps2pdf, input_path, output_path],
            capture_output=True, text=True, timeout=120)
        if result.returncode != 0:
            raise RuntimeError(f"wps2pdf failed: {result.stderr.strip()}")
        return True
    wps_bin = shutil.which("wps")
    if wps_bin:
        print("WPS CLI tool not found. Attempting to open WPS for export...", file=sys.stderr)
        print(f"Please open '{input_path}' in WPS and export to PDF manually.", file=sys.stderr)
        subprocess.Popen([wps_bin, input_path])
        raise RuntimeError("WPS opened in GUI mode. Please export to PDF manually.")
    raise RuntimeError("WPS Office not found")


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

    print(file=sys.stderr)
    print("Error: No conversion engine available.", file=sys.stderr)
    if last_error:
        print(f"  Last error: {last_error}", file=sys.stderr)
    print(file=sys.stderr)
    print("Install one of the following compatible office suites:", file=sys.stderr)
    print("", file=sys.stderr)
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

Auto-detects available engines: LibreOffice > WPS Office.
        """)
    parser.add_argument("input", help="Input .doc file path")
    parser.add_argument("-o", "--output", required=True,
                        help="Output PDF file path")
    args = parser.parse_args()

    convert(args.input, args.output)


if __name__ == "__main__":
    main()
