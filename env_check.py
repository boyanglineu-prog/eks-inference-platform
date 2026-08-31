#!/usr/bin/env python3
# env_check.py
import sys
import subprocess

print("=" * 50)
print("PYTHON ENVIRONMENT DIAGNOSTIC")
print("=" * 50)

# Python version & location
print(f"\n[Python]")
print(f"  Version    : {sys.version}")
print(f"  Executable : {sys.executable}")
print(f"  Path       : {sys.path[:3]}...")  # first 3 entries

# Pip version
print(f"\n[Pip]")
subprocess.run([sys.executable, "-m", "pip", "--version"])

# Virtual env detection
import os

venv = os.environ.get("VIRTUAL_ENV") or os.environ.get("CONDA_DEFAULT_ENV")
print(f"\n[Virtual Environment]")
print(f"  Active env : {venv if venv else 'None detected (using system Python)'}")

# Installed packages
print(f"\n[Installed Packages]")
subprocess.run([sys.executable, "-m", "pip", "list"])
