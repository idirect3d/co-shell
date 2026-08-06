"""Decode \\uXXXX escapes inside Go double-quoted strings to literal UTF-8.

Preserves:
- backtick (raw string) content: \\u is a literal there
- escaped backslash before u (\\u): stays literal
- line/block comments
"""
import re
import sys

U_RE = re.compile(r'\\u([0-9a-fA-F]{4})')


def process(src: str) -> str:
    out = []
    i = 0
    n = len(src)
    # in_raw tracks backtick string state
    in_raw = False
    # in_dq tracks double-quoted string state
    in_dq = False
    while i < n:
        ch = src[i]
        if in_raw:
            out.append(ch)
            if ch == '`':
                in_raw = False
            i += 1
            continue
        if in_dq:
            if ch == '\\':
                # possible \\uXXXX or escaped char
                m = U_RE.match(src, i)
                if m and (i == 0 or src[i - 1] != '\\'):
                    cp = int(m.group(1), 16)
                    out.append(chr(cp))
                    i += 6
                    continue
                # escaped sequence (e.g. \\n, \\", \\\\, \\u)
                if i + 1 < n:
                    out.append(ch)
                    out.append(src[i + 1])
                    i += 2
                    continue
                out.append(ch)
                i += 1
                continue
            out.append(ch)
            if ch == '"':
                in_dq = False
            i += 1
            continue
        # outside strings
        if ch == '`':
            in_raw = True
            out.append(ch)
            i += 1
            continue
        if ch == '"':
            in_dq = True
            out.append(ch)
            i += 1
            continue
        if ch == '/' and i + 1 < n and src[i + 1] == '/':
            # line comment: copy to end of line
            j = src.find('\n', i)
            if j < 0:
                j = n
            out.append(src[i:j])
            i = j
            continue
        if ch == '/' and i + 1 < n and src[i + 1] == '*':
            j = src.find('*/', i + 2)
            if j < 0:
                j = n
            else:
                j += 2
            out.append(src[i:j])
            i = j
            continue
        out.append(ch)
        i += 1
    return ''.join(out)


def main() -> int:
    for fname in sys.argv[1:]:
        with open(fname, 'r', encoding='utf-8') as f:
            src = f.read()
        new = process(src)
        if new != src:
            with open(fname, 'w', encoding='utf-8') as f:
                f.write(new)
            print(f'updated: {fname}')
        else:
            print(f'unchanged: {fname}')
    return 0


if __name__ == '__main__':
    raise SystemExit(main())