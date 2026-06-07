#!/usr/bin/env python3
"""WPS to PDF Converter.

Converts .wps (WPS Office Writer format) files to PDF.
Requires WPS Office or LibreOffice installed.

Usage:
    python3 bin/wps2pdf.py input.wps -o output.pdf
    python3 bin/wps2pdf.py input.wps -o output.pdf --engine wps
"""

import argparse
import os
import shutil
import subprocess
import sys


def check_command(cmd):
    """Check if a command is available on the system."""
    return shutil.which(cmd) is not None


def convert_with_wps(input_path, output_path):
    """Convert using WPS Office command line."""
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


def convert(input_path, output_path, engine="auto"):
    """Convert a .wps file to PDF.

    Args:
        input_path: Path to the input .wps file.
        output_path: Path for the output PDF file.
        engine: Conversion engine: "auto" (default, WPS first), "wps", "libreoffice".

    Returns:
        True on success.
    """
    if not os.path.exists(input_path):
        print(f"Error: File not found: {input_path}", file=sys.stderr)
        sys.exit(1)

    ext = os.path.splitext(input_path)[1].lower()
    if ext not in (".wps", ".wpt"):
        print(f"Warning: Unexpected file extension '{ext}'. Attempting conversion anyway.",
              file=sys.stderr)

    if engine == "auto":
        # WPS first, LibreOffice fallback
        candidates = []
        if check_command("wps2pdf") or check_command("wps"):
            candidates.append(("WPS", convert_with_wps))
        if check_command("soffice") or check_command("libreoffice"):
            candidates.append(("LibreOffice", convert_with_libreoffice))

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
    elif engine == "wps":
        if not (check_command("wps2pdf") or check_command("wps")):
            print("Error: WPS Office not found", file=sys.stderr)
            print_hints()
            sys.exit(1)
        convert_with_wps(input_path, output_path)
        print(f"Done: {output_path}")
        return True
    elif engine == "libreoffice":
        if not (check_command("soffice") or check_command("libreoffice")):
            print("Error: LibreOffice not found", file=sys.stderr)
            print_hints()
            sys.exit(1)
        convert_with_libreoffice(input_path, output_path)
        print(f"Done: {output_path}")
        return True
    else:
        print(f"Error: Unknown engine '{engine}'", file=sys.stderr)
        sys.exit(1)


def print_hints():
    """Print cross-platform installation hints, WPS first."""
    print(file=sys.stderr)
    print("Recommendation: Install WPS Office (free, best WPS file compatibility)", file=sys.stderr)
    if sys.platform == "darwin":
        print("  macOS: brew install --cask wpsoffice", file=sys.stderr)
    elif sys.platform == "win32":
        print("  Windows: winget install Kingsoft.WPSOffice", file=sys.stderr)
    elif sys.platform == "linux":
        print("  Linux: wget https://wps.com/linux/wps.deb && sudo dpkg -i wps.deb", file=sys.stderr)
    print("  Download: https://www.wps.com/", file=sys.stderr)
    print(file=sys.stderr)
    print("  Alternative: LibreOffice", file=sys.stderr)
    if sys.platform == "darwin":
        print("    brew install --cask libreoffice", file=sys.stderr)
    elif sys.platform == "win32":
        print("    winget install TheDocumentFoundation.LibreOffice", file=sys.stderr)
    elif sys.platform == "linux":
        print("    sudo apt install libreoffice", file=sys.stderr)
    print(file=sys.stderr)


def main():
    """Entry point."""
    parser = argparse.ArgumentParser(
        description="Convert WPS (.wps) files to PDF format.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s document.wps -o document.pdf
  %(prog)s document.wps -o document.pdf --engine wps

Supported engines:
  wps          - WPS Office command line (recommended, default)
  libreoffice  - LibreOffice headless mode
        """)
    parser.add_argument("input", help="Input .wps file path")
    parser.add_argument("-o", "--output", required=True,
                        help="Output PDF file path")
    parser.add_argument("--engine", choices=["auto", "wps", "libreoffice"],
                        default="auto",
                        help="Conversion engine (default: auto-detect, WPS first)")
    args = parser.parse_args()

    convert(args.input, args.output, args.engine)


if __name__ == "__main__":
    main()