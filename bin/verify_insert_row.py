#!/usr/bin/env python3
"""Verify the insert row test output file."""
import zipfile, re, sys, os

os.chdir(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
path = 'work/research/2026_calendar_insert_row_test.xlsx'

m = zipfile.ZipFile(path)
s = m.read('xl/worksheets/sheet1.xml').decode()
ss = m.read('xl/sharedStrings.xml').decode()
strs = re.findall(r'<t[^>]*>([^<]*)</t>', ss)

print(f"File: {path}")
print(f"SST count: {len(strs)}")
print(f"Sheet rows: {s.count('<row ')}")
print(f"Zip entries: {len(m.namelist())}")
print()

# Show key cells
cell_checks = {
    'A1': (0,0), 'A3': (0,2), 'B3': (1,2),
    'A11': (0,10), 'A13': (0,12)
}
for label, (c,r) in cell_checks.items():
    matches = re.findall(r'<c[^>]*?r="(\w+)"[^>]*>(.*?)</c>', s, re.DOTALL)
    for ref, body in matches:
        if ref == label:
            vm = re.search(r'<v>([^<]*)</v>', body)
            val = vm.group(1) if vm else ''
            if 't="s"' in body:
                idx = int(val)
                resolved = strs[idx] if 0 <= idx < len(strs) else f'?{val}?'
                print(f'{label}: SST[{val}]="{resolved}"')
            else:
                print(f'{label}: inline="{val}"')

# Check for duplicate zip entries
from collections import Counter
all_entries = [n for n in m.namelist()]
dupes = [n for n, cnt in Counter(all_entries).items() if cnt > 1]
if dupes:
    print(f"\n⚠ Duplicate entries: {dupes}")
else:
    print(f"\n✅ No duplicate entries")

orig = zipfile.ZipFile('work/research/2026_calendar.xlsx')
print(f"\nOriginal: {len(orig.namelist())} entries, {len(strs)} SST strings")
for n in orig.namelist():
    if n.startswith('xl/theme') or n.startswith('docProps'):
        info = orig.getinfo(n)
        info2 = m.getinfo(n) if n in all_entries else None
        if info2:
            print(f"  {n}: {info.file_size} -> {info2.file_size} bytes")
orig.close()
m.close()