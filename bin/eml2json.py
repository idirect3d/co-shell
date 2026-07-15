#!/usr/bin/env python3
"""EML to JSON Converter.

Parses EML email files and extracts:
- Headers: subject, from, to, cc, bcc, date
- Body: plain text + HTML
- Attachments: saved as separate files

Output: a folder named after the email date + subject prefix,
containing metadata.json and all attachment files.

Dependencies: Python 3 standard library only (no additional pip installs).

Usage:
    python3 bin/eml2json.py mail.eml
    python3 bin/eml2json.py mail.eml -o /path/to/output
    python3 bin/eml2json.py mail.eml --no-html
    python3 bin/eml2json.py mail.eml --text-only
    python3 bin/eml2json.py mail.eml --no-attach
"""

import argparse
import email
import email.policy
import json
import os
import re
import sys
import time
from datetime import datetime, timezone, timedelta
from email.header import decode_header
from email.utils import parsedate_tz, mktime_tz
from pathlib import Path


# ── helpers ──────────────────────────────────────────────────────────────

def _decode_mime_header(value):
    """Decode a MIME-encoded header value to plain string.
    e.g. '=?UTF-8?B?5Lit5paH?=' -> '中文'
    """
    if not value:
        return ""
    parts = []
    for chunk, charset in decode_header(value):
        if isinstance(chunk, bytes):
            try:
                parts.append(chunk.decode(charset or "utf-8", errors="replace"))
            except (LookupError, UnicodeDecodeError):
                parts.append(chunk.decode("utf-8", errors="replace"))
        else:
            parts.append(chunk)
    return "".join(parts)


def _sanitize_filename(name, max_len=128):
    """Remove unsafe characters from a filename, truncate if too long."""
    if not name:
        return "unnamed"
    # Strip path separators and control chars
    name = re.sub(r'[/\\]', '_', name)
    name = re.sub(r'[\x00-\x1f\x7f]', '', name)
    name = name.strip('. ')
    if not name:
        name = "unnamed"
    if len(name) > max_len:
        base, ext = os.path.splitext(name)
        ext = ext[:10]  # limit extension length
        max_base = max_len - len(ext) - 3
        if max_base < 1:
            max_base = 1
        name = base[:max_base] + "..." + ext
    return name


def _parse_date(date_str):
    """Parse email date string to ISO 8601 string + Unix timestamp.

    Handles RFC 2822 with various quirks (Chinese weekday names,
    missing seconds, non-standard timezone names, etc.).
    Returns (iso_str, unix_ts) or (None, None) if unparsable.
    """
    if not date_str:
        return None, None

    # Strip known non-standard timezone names from parentheses
    # e.g. "(CST)" -> "", but keep +0800
    cleaned = re.sub(r'\s*\([^)]*\)\s*$', '', date_str.strip())

    # Try email.utils.parsedate_tz first (handles RFC 2822 natively)
    try:
        parsed = parsedate_tz(cleaned)
        if parsed:
            ts = mktime_tz(parsed)
            if ts is not None:
                dt = datetime.fromtimestamp(ts, tz=timezone.utc)
                return dt.isoformat(), int(ts)
    except (ValueError, OverflowError, OSError):
        pass

    # Fallback: try common patterns
    # Remove Chinese weekday names: "星期X" / "周X"
    cleaned = re.sub(r'星期[一二三四五六日天]', '', cleaned)
    cleaned = re.sub(r'周[一二三四五六日天]', '', cleaned)

    # Normalize whitespace
    cleaned = re.sub(r'\s+', ' ', cleaned).strip()

    # Try common date formats
    formats = [
        "%a, %d %b %Y %H:%M:%S %z",
        "%a, %d %b %Y %H:%M:%S %Z",
        "%d %b %Y %H:%M:%S %z",
        "%d %b %Y %H:%M:%S %Z",
        "%a, %d %b %Y %H:%M %z",
        "%d %b %Y %H:%M %z",
        "%Y-%m-%d %H:%M:%S",
        "%Y-%m-%dT%H:%M:%S",
    ]
    for fmt in formats:
        try:
            dt = datetime.strptime(cleaned, fmt)
            if dt.tzinfo is None:
                dt = dt.replace(tzinfo=timezone.utc)
            return dt.isoformat(), int(dt.timestamp())
        except ValueError:
            continue

    return None, None


def _make_folder_name(date_ts, subject, max_len=200):
    """Create a folder name from email date and subject.

    Format: YYYY-MM-DD_HH-MM-SS_subject_prefix
    """
    if date_ts:
        dt = datetime.fromtimestamp(date_ts)
        prefix = dt.strftime("%Y-%m-%d_%H-%M-%S")
    else:
        prefix = "unknown-date"

    # Add a sanitized subject fragment
    if subject:
        safe_subj = _sanitize_filename(subject, 80)
        name = f"{prefix}_{safe_subj}"
    else:
        name = prefix

    if len(name) > max_len:
        name = name[:max_len].rstrip('_')

    return name


# ── MIME parsing ─────────────────────────────────────────────────────────

def _collect_addresses(msg, header_name):
    """Collect email addresses from a header, return list of {name, address}."""
    values = msg.get_all(header_name, [])
    result = []
    for v in values:
        addrs = email.utils.getaddresses([str(v)])
        for display_name, addr in addrs:
            display_name = _decode_mime_header(display_name)
            if addr:
                result.append({
                    "name": display_name or "",
                    "address": addr,
                })
    return result


def _get_single_address(msg, header_name):
    """Get a single email address from a header."""
    v = msg.get(header_name, "")
    if not v:
        return {}
    addrs = email.utils.getaddresses([str(v)])
    if addrs:
        display_name = _decode_mime_header(addrs[0][0])
        return {
            "name": display_name or "",
            "address": addrs[0][1],
        }
    return {}


def _parse_body(msg, no_html=False, text_only=False):
    """Recursively extract text/plain and text/html bodies from MIME parts.

    Returns (text_body, html_body).
    """
    text_parts = []
    html_parts = []

    def walk(part):
        ctype = part.get_content_type()
        cdisp = part.get_content_disposition()

        # Skip attachments
        if cdisp and cdisp.lower() in ("attachment", "inline"):
            return

        if ctype == "text/plain":
            try:
                payload = part.get_payload(decode=True)
                if payload:
                    charset = part.get_content_charset() or "utf-8"
                    try:
                        text_parts.append(payload.decode(charset, errors="replace"))
                    except LookupError:
                        text_parts.append(payload.decode("utf-8", errors="replace"))
            except Exception:
                pass

        elif ctype == "text/html" and not no_html:
            try:
                payload = part.get_payload(decode=True)
                if payload:
                    charset = part.get_content_charset() or "utf-8"
                    try:
                        html_parts.append(payload.decode(charset, errors="replace"))
                    except LookupError:
                        html_parts.append(payload.decode("utf-8", errors="replace"))
            except Exception:
                pass

        elif part.get_content_maintype() == "multipart":
            for sub in part.get_payload():
                if isinstance(sub, email.message.Message):
                    walk(sub)

    walk(msg)

    text_body = "\n".join(text_parts) if text_parts else ""
    html_body = "\n".join(html_parts) if html_parts else ""
    return text_body, html_body


def _collect_attachments(msg):
    """Collect attachments from MIME parts.

    Returns list of dicts with {filename, content_type, size, data, content_id}.
    """
    attachments = []

    def walk(part):
        cdisp = part.get_content_disposition()
        if not cdisp or cdisp.lower() not in ("attachment", "inline"):
            if part.get_content_maintype() == "multipart":
                for sub in part.get_payload():
                    if isinstance(sub, email.message.Message):
                        walk(sub)
            return

        filename = part.get_filename()
        if filename:
            filename = _decode_mime_header(filename)
        if not filename:
            # Generate a name from content type
            ext_map = {
                "text/plain": ".txt", "text/html": ".html",
                "application/pdf": ".pdf", "image/png": ".png",
                "image/jpeg": ".jpg", "image/gif": ".gif",
            }
            ext = ext_map.get(part.get_content_type(), ".bin")
            filename = f"attachment_{len(attachments)}{ext}"

        filename = _sanitize_filename(filename)

        try:
            data = part.get_payload(decode=True)
        except Exception:
            data = None

        att = {
            "filename": filename,
            "content_type": part.get_content_type(),
            "size": len(data) if data else 0,
            "content_id": part.get("Content-ID", "").strip("<>"),
        }

        if data:
            att["data"] = data

        attachments.append(att)

    walk(msg)
    return attachments


# ── main ──────────────────────────────────────────────────────────────────

def parse_eml(eml_path, no_html=False, text_only=False, no_attach=False):
    """Parse an EML file and return structured metadata + attachment data.

    Returns dict:
        {
            "metadata": {...},
            "attachments_data": [{"filename": ..., "data": bytes}, ...],
        }
    """
    with open(eml_path, "rb") as f:
        msg = email.message_from_binary_file(f, policy=email.policy.compat32)

    # ── headers ──
    subject = _decode_mime_header(msg.get("Subject", ""))
    from_addr = _get_single_address(msg, "From")
    to_list = _collect_addresses(msg, "To")
    cc_list = _collect_addresses(msg, "Cc")
    bcc_list = _collect_addresses(msg, "Bcc")

    date_str_raw = msg.get("Date", "")
    date_iso, date_ts = _parse_date(date_str_raw)

    # ── body ──
    text_body, html_body = _parse_body(msg, no_html=no_html, text_only=text_only)

    # ── attachments metadata ──
    attachments = _collect_attachments(msg) if not no_attach else []

    # ── raw headers (selected) ──
    raw_headers = {}
    important_headers = [
        "Message-ID", "In-Reply-To", "References", "X-Mailer",
        "X-Originating-IP", "X-Priority", "MIME-Version",
    ]
    for h in important_headers:
        v = msg.get(h, "")
        if v:
            raw_headers[h] = _decode_mime_header(v)

    # ── build metadata ──
    metadata = {
        "subject": subject,
        "from": from_addr,
        "to": to_list,
        "cc": cc_list,
        "bcc": bcc_list,
        "date_raw": date_str_raw,
        "date": date_iso,
        "date_ts": date_ts,
        "text_body": text_body,
        "html_body": html_body if not text_only else "",
        "attachments": [
            {
                "filename": a["filename"],
                "content_type": a["content_type"],
                "size": a["size"],
                "content_id": a["content_id"],
            }
            for a in attachments
        ],
        "headers": raw_headers,
        "source_file": str(eml_path),
        "encoding_warning": None,
    }

    # Check for encoding issues
    if text_body and "\ufffd" in text_body:
        metadata["encoding_warning"] = (
            "Some characters could not be decoded properly. "
            "The text may contain replacement characters (U+FFFD)."
        )

    # Separate data for files
    attachments_data = []
    for a in attachments:
        if "data" in a and a["data"]:
            attachments_data.append({
                "filename": a["filename"],
                "data": a["data"],
            })

    return {
        "metadata": metadata,
        "attachments_data": attachments_data,
    }


def main():
    parser = argparse.ArgumentParser(
        description="Parse EML email file and extract headers, body, and attachments.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s mail.eml
  %(prog)s mail.eml -o /path/to/output
  %(prog)s mail.eml --no-html
  %(prog)s mail.eml --text-only
  %(prog)s mail.eml --no-attach
        """,
    )
    parser.add_argument("input", help="Path to the .eml file")
    parser.add_argument("-o", "--output-dir", help="Output root directory (default: same directory as input file)")
    parser.add_argument("--no-html", action="store_true", help="Skip extracting HTML body")
    parser.add_argument("--text-only", action="store_true", help="Skip HTML body in output JSON")
    parser.add_argument("--no-attach", action="store_true", help="Skip extracting attachments")

    args = parser.parse_args()

    input_path = Path(args.input)
    if not input_path.exists():
        print(f"Error: file not found: {input_path}", file=sys.stderr)
        sys.exit(1)

    if not input_path.suffix.lower() == ".eml":
        print(f"Warning: file does not have .eml extension: {input_path}", file=sys.stderr)

    # Parse
    result = parse_eml(
        input_path,
        no_html=args.no_html,
        text_only=args.text_only,
        no_attach=args.no_attach,
    )
    metadata = result["metadata"]
    attachments_data = result["attachments_data"]

    # Determine output folder
    if args.output_dir:
        root_dir = Path(args.output_dir)
    else:
        root_dir = input_path.parent

    folder_name = _make_folder_name(metadata["date_ts"], metadata["subject"])
    output_dir = root_dir / folder_name

    # Create output directory
    output_dir.mkdir(parents=True, exist_ok=True)

    # Write metadata.json
    meta_path = output_dir / "metadata.json"
    with open(meta_path, "w", encoding="utf-8") as f:
        json.dump(metadata, f, ensure_ascii=False, indent=2)
    print(f"  metadata: {meta_path}")

    # Write attachments
    for att_data in attachments_data:
        att_path = output_dir / att_data["filename"]
        # Handle duplicate filenames
        counter = 1
        while att_path.exists():
            base, ext = os.path.splitext(att_data["filename"])
            att_path = output_dir / f"{base}_{counter}{ext}"
            counter += 1
        with open(att_path, "wb") as f:
            f.write(att_data["data"])
        print(f"  attachment: {att_path} ({len(att_data['data'])} bytes)")

    # Summary
    print(f"\nOutput: {output_dir}/")
    print(f"  metadata.json - email metadata ({len(attachments_data)} attachments)")
    if attachments_data:
        total_size = sum(len(a["data"]) for a in attachments_data)
        print(f"  attachments total: {total_size} bytes")

    # Return path for script consumers
    return str(output_dir)


if __name__ == "__main__":
    main()