#!/usr/bin/env python3
"""Compare zip contents of original and generated xlsx files."""
import zipfile, os

os.chdir(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
work = 'work/research'

orig = zipfile.ZipFile(f'{work}/2026_calendar.xlsx')
mod = zipfile.ZipFile(f'{work}/2026_calendar_auto_test_output.xlsx')

orig_names = set(orig.namelist())
mod_names = set(mod.namelist())

print(f"Original files ({len(orig_names)}):")
for n in sorted(orig_names):
    info = orig.getinfo(n)
    print(f"  {n:50s} {info.file_size:>6d} bytes")

print(f"\nOutput files ({len(mod_names)}):")
for n in sorted(mod_names):
    info = mod.getinfo(n)
    print(f"  {n:50s} {info.file_size:>6d} bytes")

print(f"\nFiles in original but NOT in output:")
for n in sorted(orig_names - mod_names):
    info = orig.getinfo(n)
    print(f"  {n:50s} {info.file_size:>6d} bytes")

print(f"\nFiles in output but NOT in original:")
for n in sorted(mod_names - orig_names):
    info = mod.getinfo(n)
    print(f"  {n:50s} {info.file_size:>6d} bytes")

# Compare same-file size differences
print(f"\nSame files with size differences:")
for n in sorted(orig_names & mod_names):
    osz = orig.getinfo(n).file_size
    msz = mod.getinfo(n).file_size
    if osz != msz:
        print(f"  {n:50s} {osz:>6d} -> {msz:>6d} bytes (diff {msz-osz:>+6d})")

orig.close()
mod.close()