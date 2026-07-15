#!/usr/bin/env python3
"""Run the xlsx round-trip test and copy output to work/research/ for inspection."""
import subprocess, os, shutil

os.chdir(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
subprocess.run(["go", "test", "./xlsx/", "-run", "TestPreserveFormatOnEdit", "-v"], check=True)