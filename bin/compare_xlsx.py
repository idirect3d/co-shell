#!/usr/bin/env python3
"""Compare original xlsx with test output, row by row."""
import zipfile, re, os

os.chdir(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
work = 'work/research'

for fname in os.listdir(work):
    if fname.endswith('.xlsx') and 'original' not in fname and fname not in ('2026_calendar.xlsx',):
        print(f"Found output: {fname}")

orig = zipfile.ZipFile(f'{work}/2026_calendar.xlsx')
mod = zipfile.ZipFile(f'{work}/2026_calendar_auto_test_output.xlsx')

orig_s = orig.read('xl/worksheets/sheet1.xml').decode()
mod_s = mod.read('xl/worksheets/sheet1.xml').decode()

def analyze_rows(xml, label):
    rows = re.findall(r'<row[^>]*>(.*?)</row>', xml, re.DOTALL)
    row_map = {}
    for r_tag, r_body in zip(re.findall(r'<row[^>]*>', xml), rows):
        m = re.search(r'r="(\d+)"', r_tag)
        rn = int(m.group(1)) if m else 0
        cells = re.findall(r'<c[^>]*>', r_body)
        style = sum(1 for c in cells if 's="' in c[:50])
        row_map[rn] = {'cells': len(cells), 'styled': style, 'has_ht': 'ht="' in r_tag}
    return row_map

orig_rows = analyze_rows(orig_s, 'original')
mod_rows = analyze_rows(mod_s, 'modified')

print(f"Original rows: {len(orig_rows)}, Modified rows: {len(mod_rows)}")
print()

issues = []
for rn in sorted(set(list(orig_rows.keys()) + list(mod_rows.keys()))):
    o = orig_rows.get(rn)
    m = mod_rows.get(rn)
    if o is None:
        issues.append(f"Row {rn}: MISSING from original?")
        continue
    if m is None:
        issues.append(f"Row {rn}: MISSING from output ({o['cells']} cells, {o['styled']} styled)")
        continue
    diffs = []
    if o['cells'] != m['cells']: diffs.append(f"cells {o['cells']}→{m['cells']}")
    if o['styled'] != m['styled']: diffs.append(f"styled {o['styled']}→{m['styled']}")
    if o['has_ht'] != m['has_ht']: diffs.append("ht lost")
    if diffs:
        issues.append(f"Row {rn}: {'; '.join(diffs)}")

if issues:
    print("⚠ ISSUES FOUND:")
    for i in issues:
        print(f"  {i}")
else:
    print("✅ All rows match between original and output")