#!/usr/bin/env python3
"""DOCX to PDF Converter.

Converts .docx (Word Open XML) files to PDF format.
Supports printing, document sharing, and further conversion to PNG
for multimodal LLM visual recognition.

Requires one of the following office suites installed on the system:
- WPS Office (recommended, free, best compatibility)
- LibreOffice (free, open-source)

Usage:
    python3 bin/docx2pdf.py input.docx -o output.pdf
    python3 bin/docx2pdf.py input.docx -o output.pdf --engine wps
"""

import argparse
import os
import shutil
import subprocess
import sys


def check_command(cmd):
    """Check if a command is available on the system."""
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
    """Convert using WPS Office command line.

    WPS provides the 'wps2pdf' command on Linux.
    On macOS/Windows, opens WPS GUI to export; if only GUI is available,
    prints instructions for manual conversion.
    """
    wps2pdf = shutil.which("wps2pdf")
    if wps2pdf:
        result = subprocess.run(
            [wps2pdf, input_path, output_path],
            capture_output=True, text=True, timeout=120)
        if result.returncode != 0:
            raise RuntimeError(f"wps2pdf failed: {result.stderr.strip()}")
        return True
    # No CLI tool found — try opening WPS GUI
    wps_bin = shutil.which("wps")
    if wps_bin:
        print("WPS CLI tool not found. Attempting to open WPS for export...", file=sys.stderr)
        print(f"Please open '{input_path}' in WPS and export to PDF manually.", file=sys.stderr)
        # Try opening the file in WPS
        subprocess.Popen([wps_bin, input_path])
        raise RuntimeError("WPS opened in GUI mode. Please export to PDF manually.")
    raise RuntimeError("WPS Office not found")


def convert(input_path, output_path, engine="auto"):
    """Convert a .docx file to PDF.

    Args:
        input_path: Path to the input .docx file.
        output_path: Path for the output PDF file.
        engine: Conversion engine: "auto" (default), "libreoffice", "wps".

    Returns:
        True on success.
    """
    if not os.path.exists(input_path):
        print(f"Error: File not found: {input_path}", file=sys.stderr)
        sys.exit(1)

    ext = os.path.splitext(input_path)[1].lower()
    if ext not in (".docx", ".doc", ".rtf", ".odt"):
        print(f"Warning: Unexpected file extension '{ext}'. Attempting conversion anyway.",
              file=sys.stderr)

    engines = {
        "libreoffice": ("LibreOffice", convert_with_libreoffice),
        "wps": ("WPS", convert_with_wps),
    }

    if engine == "auto":
        # Try LibreOffice first, then WPS
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
        print(f"Error: No conversion engine succeeded.", file=sys.stderr)
        if last_error:
            print(f"  Last error: {last_error}", file=sys.stderr)
        print_hints()
        sys.exit(1)
    else:
        name, func = engines.get(engine, (None, None))
        if name is None:
            print(f"Error: Unknown engine '{engine}'", file=sys.stderr)
            sys.exit(1)
        # Validate the engine exists
        if engine == "libreoffice" and not (check_command("soffice") or check_command("libreoffice")):
            print("Error: LibreOffice not found", file=sys.stderr)
            print_hints()
            sys.exit(1)
        if engine == "wps" and not (check_command("wps2pdf") or check_command("wps")):
            print("Error: WPS Office not found", file=sys.stderr)
            print_hints()
            sys.exit(1)
        func(input_path, output_path)
        print(f"Done: {output_path}")
        return True


def print_hints():
    """Print installation hints for the user."""
    print(file=sys.stderr)
    print("Install one of the following compatible office suites:", file=sys.stderr)
    print("", file=sys.stderr)
    print("  \u2b50 WPS Office (recommended, free, best docx/doc compatibility):", file=sys.stderr)
    print("     https://www.wps.com/", file=sys.stderr)
    print("  LibreOffice (free, open-source):", file=sys.stderr)
    print("     https://www.libreoffice.org/", file=sys.stderr)
    print(file=sys.stderr)


def main():
    """Entry point."""
    parser = argparse.ArgumentParser(
        description="Convert DOCX files to PDF format.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s report.docx -o report.pdf
  %(prog)s report.docx -o report.pdf --engine wps
  %(prog)s report.docx -o report.pdf --engine libreoffice

Supported engines:
  libreoffice  - LibreOffice headless mode (free, open-source)
  wps          - WPS Office command line (recommended, free, best compatibility)
        """)
    parser.add_argument("input", help="Input .docx file path")
    parser.add_argument("-o", "--output", required=True,
                        help="Output PDF file path")
    parser.add_argument("--engine", choices=["auto", "libreoffice", "wps"],
                        default="auto",
                        help="Conversion engine (default: auto-detect)")
    args = parser.parse_args()

    convert(args.input, args.output, args.engine)


if __name__ == "__main__":
    main()
