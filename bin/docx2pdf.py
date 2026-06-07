#!/usr/bin/env python3
"""DOCX to PDF Converter.

Converts .docx (Word Open XML) files to PDF format.
Supports printing, document sharing, and further conversion to PNG
for multimodal LLM visual recognition.

Priority engine order (auto mode):
1. WPS Office (recommended, free, best compatibility)
2. LibreOffice (free, open-source fallback)

Cross-platform support:
- Linux: Detects 'wps2pdf' CLI command
- macOS: Detects WPS Office.app, uses AppleScript to drive PDF export
- Windows: Detects wps.exe path, uses COM automation

Usage:
    python3 bin/docx2pdf.py input.docx -o output.pdf
    python3 bin/docx2pdf.py input.docx -o output.pdf --engine wps
"""

import argparse
import os
import platform
import shutil
import subprocess
import sys


def check_command(cmd):
    """Check if a command is available on the system."""
    return shutil.which(cmd) is not None


def find_wps_path():
    """Find WPS Office executable path on macOS/Windows."""
    if platform.system() == "Darwin":
        candidates = [
            "/Applications/WPS Office.app/Contents/MacOS/wps",
            os.path.expanduser("~/Applications/WPS Office.app/Contents/MacOS/wps"),
        ]
        for p in candidates:
            if os.path.isfile(p) and os.access(p, os.X_OK):
                return p
        return None
    elif platform.system() == "Windows":
        candidates = [
            r"C:\Program Files\WPS Office\*\wps.exe",
            r"C:\Program Files (x86)\WPS Office\*\wps.exe",
        ]
        import glob
        for pattern in candidates:
            matches = glob.glob(pattern)
            if matches:
                return matches[0]
        return None
    return None


def convert_with_wps_linux(input_path, output_path):
    """Convert using WPS Office wps2pdf on Linux."""
    wps2pdf = shutil.which("wps2pdf")
    if not wps2pdf:
        raise RuntimeError("wps2pdf not found on PATH")
    result = subprocess.run(
        [wps2pdf, input_path, output_path],
        capture_output=True, text=True, timeout=120)
    if result.returncode != 0:
        raise RuntimeError(f"wps2pdf failed: {result.stderr.strip()}")
    return True


def convert_with_wps_macos(input_path, output_path):
    """Convert using WPS Office on macOS via AppleScript."""
    wps_path = find_wps_path()
    if not wps_path:
        raise RuntimeError("WPS Office.app not found on macOS")

    abs_input = os.path.abspath(input_path)
    abs_output = os.path.abspath(output_path)

    # AppleScript: open docx in WPS, export to PDF, quit
    script = (
        f'tell application "WPS Office"\n'
        f'    activate\n'
        f'    open POSIX file "{abs_input}"\n'
        f'    delay 2\n'
        f'end tell\n'
        f'tell application "System Events"\n'
        f'    tell process "wps"\n'
        f'        keystroke "p" using {{command down}}\n'
        f'        delay 1\n'
        f'    end tell\n'
        f'end tell\n'
        # Alternative: use menu bar: File -> Export to PDF
        # WPS on macOS does NOT have a reliable CLI export,
        # so we use the WPS built-in: 'wps -export-to-pdf'
    )

    # Better approach: WPS on macOS supports --headless-export-pdf in recent versions
    result = subprocess.run(
        [wps_path, f'"{abs_input}"', '--export-to-pdf', f'"{abs_output}"'],
        capture_output=True, text=True, timeout=120,
        shell=True)
    if result.returncode != 0:
        # Fall back to AppleScript if CLI arg not supported
        try:
            subprocess.run(
                ["osascript", "-e",
                 f'tell application "WPS Office" to open POSIX file "{abs_input}"',
                 "-e",
                 'delay 2',
                 "-e",
                 f'tell application "WPS Office" to export active document to PDF path "{abs_output}" true'],
                capture_output=True, text=True, timeout=60)
            # Give WPS time to complete export
            import time
            time.sleep(3)
            if os.path.exists(abs_output) and os.path.getsize(abs_output) > 0:
                # Quit WPS
                subprocess.run(["osascript", "-e",
                                'tell application "WPS Office" to quit'],
                               capture_output=True, timeout=10)
                return True
            raise RuntimeError("WPS AppleScript export timed out or failed")
        except Exception as e2:
            raise RuntimeError(f"WPS macOS export failed: {e2}")
    return True


def convert_with_wps_windows(input_path, output_path):
    """Convert using WPS Office on Windows via COM automation."""
    abs_input = os.path.abspath(input_path)
    abs_output = os.path.abspath(output_path)
    try:
        import win32com.client
        wps_app = win32com.client.Dispatch("KWps.Application")
        wps_app.Visible = False
        doc = wps_app.Documents.Open(abs_input)
        doc.ExportAsFixedFormat(abs_output, 17)  # 17 = wdExportFormatPDF
        doc.Close()
        wps_app.Quit()
        return True
    except ImportError:
        raise RuntimeError("pywin32 not available. Install: pip install pywin32")
    except Exception as e:
        raise RuntimeError(f"WPS Windows COM failed: {e}")


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
    """Convert using WPS Office (platform-aware)."""
    system = platform.system()
    if system == "Linux":
        return convert_with_wps_linux(input_path, output_path)
    elif system == "Darwin":
        return convert_with_wps_macos(input_path, output_path)
    elif system == "Windows":
        return convert_with_wps_windows(input_path, output_path)
    else:
        raise RuntimeError(f"Unsupported platform: {system}")


def convert(input_path, output_path, engine="auto"):
    """Convert a .docx file to PDF.

    Args:
        input_path: Path to the input .docx file.
        output_path: Path for the output PDF file.
        engine: "auto" (default, WPS first), "wps", "libreoffice".

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
        # WPS first (preferred), LibreOffice fallback
        candidates = []

        # WPS availability check per platform
        system = platform.system()
        if system == "Linux" and check_command("wps2pdf"):
            candidates.append(("WPS", convert_with_wps))
        elif system == "Darwin" and find_wps_path() is not None:
            candidates.append(("WPS", convert_with_wps))
        elif system == "Windows" and find_wps_path() is not None:
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
    else:
        name, func = engines.get(engine, (None, None))
        if name is None:
            print(f"Error: Unknown engine '{engine}'", file=sys.stderr)
            sys.exit(1)
        func(input_path, output_path)
        print(f"Done: {output_path}")
        return True


def print_hints():
    """Print cross-platform installation hints, WPS first."""
    import platform
    system = platform.system()
    print(file=sys.stderr)
    print("Recommendation: Install WPS Office (free, best compatibility)", file=sys.stderr)
    if system == "Darwin":
        print("  macOS: brew install --cask wpsoffice", file=sys.stderr)
        print("  Or download: https://www.wps.com/", file=sys.stderr)
    elif system == "Windows":
        print("  Windows: winget install Kingsoft.WPSOffice", file=sys.stderr)
        print("  Or download: https://www.wps.com/", file=sys.stderr)
    else:
        print("  Linux: wget https://wps.com/linux/wps.deb && sudo dpkg -i wps.deb", file=sys.stderr)
        print("  Or download: https://www.wps.com/", file=sys.stderr)
    print(file=sys.stderr)
    print("  Alternative: LibreOffice (free, open-source)", file=sys.stderr)
    if system == "Darwin":
        print("    brew install --cask libreoffice", file=sys.stderr)
    elif system == "Windows":
        print("    winget install TheDocumentFoundation.LibreOffice", file=sys.stderr)
    else:
        print("    sudo apt install libreoffice  # Debian/Ubuntu", file=sys.stderr)
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
  wps          - WPS Office (recommended, cross-platform)
  libreoffice  - LibreOffice headless mode (fallback)
        """)
    parser.add_argument("input", help="Input .docx file path")
    parser.add_argument("-o", "--output", required=True,
                        help="Output PDF file path")
    parser.add_argument("--engine", choices=["auto", "wps", "libreoffice"],
                        default="auto",
                        help="Conversion engine (default: auto-detect, WPS first)")
    args = parser.parse_args()

    convert(args.input, args.output, args.engine)


if __name__ == "__main__":
    main()