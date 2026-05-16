#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
ngxlog_decode.py - Nginx日志HTTP Body转义还原过滤程序

功能：
  从标准输入读取Nginx日志，将其中各种转义序列还原为原始可读内容。
  支持通过管道与 tail -f 配合使用实现实时查看。

支持的转义序列：
  \\xXX        - 十六进制字节转义（连续的 \\xXX 会合并为UTF-8解码）
  \\uXXXX      - Unicode转义（如 \\u4f60 -> '你'）
  \\n          - 换行符
  \\r          - 回车符
  \\t          - 制表符
  \\\\          反斜杠
  \\"          - 双引号
  \\'          - 单引号

使用方式：
  tail -f /var/log/nginx/access.log | python3 ngx_decode.py

作者: co-shell
"""

import sys
import re
import os


def decode_x_escapes(text):
    r"""
    Decode \xXX escape sequences in the given text.
    Consecutive \xXX sequences are merged and decoded as UTF-8.
    Example: \xe4\xbd\xa0\xe5\xa5\xbd -> '你好'
    """
    # Match consecutive \xXX sequences
    pattern = r'(?:\\x[0-9a-fA-F]{2})+'

    def replace_x_sequence(match):
        hex_str = match.group(0)
        # Extract all hex values
        bytes_list = []
        for h in re.findall(r'\\x([0-9a-fA-F]{2})', hex_str):
            bytes_list.append(int(h, 16))

        # Try UTF-8 decoding first
        try:
            return bytes(bytes_list).decode('utf-8')
        except UnicodeDecodeError:
            # Fallback: decode byte by byte as Latin-1
            return ''.join(chr(b) for b in bytes_list)

    return re.sub(pattern, replace_x_sequence, text)


def decode_u_escapes(text):
    r"""
    Decode \uXXXX escape sequences (Unicode).
    Example: \u4f60\u597d -> '你好'
    """
    pattern = r'\\u([0-9a-fA-F]{4})'

    def replace_u(match):
        hex_val = match.group(1)
        return chr(int(hex_val, 16))

    return re.sub(pattern, replace_u, text)


def decode_common_escapes(text):
    r"""
    Decode common escape characters (\n, \r, \t, etc.).
    Order matters: \\ must be processed before \" and \'.
    """
    replacements = [
        ('\\\\', '\\'),   # backslash (must be first)
        ('\\n', '\n'),     # newline
        ('\\r', '\r'),     # carriage return
        ('\\t', '\t'),     # tab
        ('\\"', '"'),      # double quote
        ("\\'", "'"),      # single quote
    ]
    for old, new in replacements:
        text = text.replace(old, new)
    return text


def decode_all(text):
    r"""
    Apply all escape decoding steps.
    Order: \xXX -> \uXXXX -> common escapes
    """
    text = decode_x_escapes(text)
    text = decode_u_escapes(text)
    text = decode_common_escapes(text)
    return text


def print_header_info():
    """Print startup info to stderr (avoids polluting stdout)."""
    sys.stderr.write(f"[ngxlog_decode] 已启动，等待... (PID: {os.getpid()})\n")
    sys.stderr.write("[ngxlog_decode] 支持的转义: \\xXX \\uXXXX \\n \\t \\r \\\\ \\\" \\'\n")
    sys.stderr.write("[ngxlog_decode] 按 Ctrl+C 退出\n")
    sys.stderr.flush()


def main():
    """Main function: read from stdin, decode, and write to stdout."""
    if sys.stdout.isatty() and not sys.stdin.isatty():
        print_header_info()

    try:
        for line in sys.stdin:
            decoded = decode_all(line)
            sys.stdout.write(decoded)
            sys.stdout.flush()
    except KeyboardInterrupt:
        sys.stderr.write("\n[ngxlog_decode] 已退出\n")
        sys.exit(0)
    except BrokenPipeError:
        # Graceful exit when the pipe reader closes (e.g., less quits)
        sys.stderr.close()
        sys.exit(0)
    except Exception as e:
        sys.stderr.write(f"\n[ngxlog_decode] 错误: {e}\n")
        sys.exit(1)


if __name__ == '__main__':
    main()
