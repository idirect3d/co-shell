#!/usr/bin/env python3
"""Compare row 3 cell values between original and auto-test output xlsx files."""
import zipfile, re, sys, os

os.chdir(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
work = 'work/research'

orig = zipfile.ZipFile(f'{work}/2026_calendar.xlsx')
mod_fn = f'{work}/2026_calendar_auto_test_output.xlsx'
mod = zipfile.ZipFile(mod_fn)

os_ = orig.read('xl/worksheets/sheet1.xml').decode()
ms = mod.read('xl/worksheets/sheet1.xml').decode()

# Build SST lookup from original
ss_xml = orig.read('xl/sharedStrings.xml').decode()
si_items = re.findall(r'<si>(.*?)</si>', ss_xml, re.DOTALL)
orig_sst = []
for si in si_items:
    texts = re.findall(r'<t[^>]*>([^<]*)</t>', si)
    orig_sst.append(''.join(texts))

print(f"Original SST count: {len(orig_sst)}")
for i, s in enumerate(orig_sst):
    print(f"  [{i}] \"{s}\"")

# Extract row 3 from both files
for label, xml in [('ORIG', os_), ('MOD', ms)]:
    print(f'\n=== {label} ===')
    for rxml in re.findall(r'<row[^>]*>.*?</row>', xml, re.DOTALL):
        rn = int(re.search(r'r="(\d+)"', rxml).group(1))
        if rn != 3:
            continue
        cells = re.findall(r'<c[^>]*?r="(\w+)"[^>]*>(.*?)</c>', rxml, re.DOTALL)
        for ref, body in cells[:7]:
            vm = re.search(r'<v>([^<]*)</v>', body)
            v = vm.group(1) if vm else ''
            idx = int(v) if v.isdigit() else -1
            resolved = orig_sst[idx] if 0 <= idx < len(orig_sst) else v
            print(f'  {ref}: <v>{v}</v> → SST[{idx}]="{resolved}"')

orig.close()
mod.close()