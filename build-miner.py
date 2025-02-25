#!/usr/bin/env python3
"""
This script builds the r5miner binary using Golang.
It is compatible with Windows, Linux, and macOS.
"""

import subprocess
import sys
import os

def handle_error():
    print("Error: Failed to build r5miner.")
    sys.exit(1)

def clean_cache():
    print("Cleaning build cache...")
    subprocess.run(["go", "clean", "-cache"], check=True)

def build_r5miner():
    print("Building r5miner. Please wait...")
    clean_cache()
    # Determine output file name based on OS.
    output = "./build/bin/r5miner"
    if os.name == "nt":
        output += ".exe"
    # Assumes the r5miner main package is in ./cmd/r5miner.
    result = subprocess.run(
        ["go", "build", "-ldflags=-extldflags=-static-libgcc", "-o", output, "./cmd/r5miner"],
        check=False
    )
    if result.returncode != 0:
        handle_error()
    print("r5miner built successfully.")

if __name__ == "__main__":
    build_r5miner()
