# This script builds R5 using Golang and GCC
# This script is copatible with Windows, Linux, and macOS

import subprocess
import sys
import os

def handle_error():
    print("Error: Script failed to build R5.")
    sys.exit(1)

def clean_cache():
    print("Cleaning build cache...")
    subprocess.run(["go", "clean", "-cache"], check=True)

def build_r5():
    print("Building R5. Please wait...")
    clean_cache()
    result = subprocess.run(["go", "run", "build/ci.go", "install", "./cmd/r5", "-v", "-x"], check=False)
    if result.returncode != 0:
        handle_error()
    print("Build finished successfully.")

if __name__ == "__main__":
    build_r5()
