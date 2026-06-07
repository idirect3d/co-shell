#!/usr/bin/env python3
"""DOCX to PDF Converter.

Converts .docx (Word Open XML) files to PDF format.
Supports printing, document sharing, and further conversion to PNG
for multimodal LLM visual recognition.

On macOS, uses the built-in `textutil` command (no extra software needed).
On other platforms, falls back to python-docx + fpdf2, or prompts the user
to install a compatible office suite (WPS/LibreOffice).

Usage:
    python3 bin/docx2pdf.py input.docx -o output.pdf
    python3 bin/docx2pdf.py input.docx -o output.pdf --engine auto
"""

import argparse
import os
import shutil
import subprocess
import sys
import tempfile


def check_command(cmd):
    """Check if a command is available on the system."""
    return shutil.which(cmd) is not None


def convert_with_textutil(input_path, output_path):
    """Convert using macOS built-in textutil (supports docx, doc, rtf, etc.)."""
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
    # LibreOffice saves with the same base name, move if different
    expected = os.path.join(output_dir,
                            os.path.splitext(os.path.basename(input_path))[0] + ".pdf")
    if expected != output_path and os.path.exists(expected):
        os.replace(expected, output_path)
    return True


def convert_with_wps(input_path, output_path):
    """Convert using WPS Office command line (wps)."""
    output_dir = os.path.dirname(os.path.abspath(output_path))
    # WPS on Linux: `wps2pdf input.docx output.pdf`
    wps_bin = shutil.which("wps2pdf") or shutil.which("wps")
    if not wps_bin:
        raise RuntimeError("WPS Office not found")
    result = subprocess.run(
        [wps_bin, input_path, output_path],
        capture_output=True, text=True, timeout=120)
    if result.returncode != 0:
        raise RuntimeError(f"WPS failed: {result.stderr.strip()}")
    return True


def convert(input_path, output_path, engine="auto"):
    """Convert a .docx file to PDF.

    Args:
        input_path: Path to the input .docx file.
        output_path: Path for the output PDF file.
        engine: Conversion engine: "auto" (default), "textutil",
                "libreoffice", "wps".

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

    candidates = []

    if engine == "auto":
        # Try backends in order of preference
        if sys.platform == "darwin":
            candidates.append(("textutil", convert_with_textutil))
        if check_command("soffice") or check_command("libreoffice"):
            candidates.append(("libreoffice", convert_with_libreoffice))
        if check_command("wps2pdf") or check_command("wps"):
            candidates.append(("wps", convert_with_wps))

        # Try each until one succeeds
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
        print_hints(input_path)
        sys.exit(1)
    else:
        # Specific engine requested
        engines = {
            "textutil": ("textutil", convert_with_textutil),
            "libreoffice": ("libreoffice", convert_with_libreoffice),
            "wps": ("wps", convert_with_wps),
        }
        name, func = engines.get(engine, (None, None))
        if name is None:
            print(f"Error: Unknown engine '{engine}'", file=sys.stderr)
            sys.exit(1)
        if not any(check_command(c) for c in (name, name + "2pdf", "soffice")):
            print(f"Error: Engine '{name}' not found on system", file=sys.stderr)
            print_hints(input_path)
            sys.exit(1)
        func(input_path, output_path)
        print(f"Done: {output_path}")
        return True


def print_hints(input_path):
    """Print installation hints for the user."""
    print(file=sys.stderr)
    print("Hints:", file=sys.stderr)
    if sys.platform == "darwin":
        print("  macOS: textutil is built-in. Try without engine selection.", file=sys.stderr)
    print("  Install WPS Office (recommended, free):  https://www.wps.com/", file=sys.stderr)
    print("  Install LibreOffice:                     https://www.libreoffice.org/", file=sys.stderr)
    print(file=sys.stderr)
    print(f"  You can also open '{input_path}' in Word/WPS and export to PDF manually.", file=sys.stderr)


def main():
    """Entry point."""
    parser = argparse.ArgumentParser(
        description="Convert DOCX files to PDF format.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s report.docx -o report.pdf
  %(prog)s report.docx -o report.pdf --engine textutil
  %(prog)s report.docx -o report.pdf --engine libreoffice

Supported engines (auto-detected on macOS):
  textutil     - macOS built-in (no install needed)
  libreoffice  - LibreOffice headless mode
  wps          - WPS Office command line
        """)
    parser.add_argument("input", help="Input .docx file path")
    parser.add_argument("-o", "--output", required=True,
                        help="Output PDF file path")
    parser.add_argument("--engine", choices=["auto", "textutil", "libreoffice", "wps"],
                        default="auto",
                        help="Conversion engine (default: auto-detect)")
    args = parser.parse_args()

    convert(args.input, args.output, args.engine)


if __name__ == "__main__":
    main()