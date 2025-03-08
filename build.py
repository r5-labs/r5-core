#!/usr/bin/env python3
# Copyright 2025 R5
# This file is part of the R5 Core library.
#
# This software is provided "as is", without warranty of any kind,
# express or implied, including but not limited to the warranties
# of merchantability, fitness for a particular purpose and
# noninfringement. In no event shall the authors or copyright
# holders be liable for any claim, damages, or other liability,
# whether in an action of contract, tort or otherwise, arising
# from, out of or in connection with the software or the use or
# other dealings in the software.

import argparse
import os
import sys
import subprocess
import shutil

def handle_error(msg):
    print(f"Error: {msg}")
    sys.exit(1)

# CORE BUILDING FUNCTION
# Builds only the client binary, without any additional tools or the R5 Relayer

def clean_cache():
    print("Cleaning build cache...")
    subprocess.run(["go", "clean", "-cache"], cwd="client", check=True)

def build_core():
    print("Building R5 core binary. Please wait...")
    clean_cache()
    result = subprocess.run(["go", "run", "build/ci.go", "install", "./cmd/r5"], cwd="client", check=False)
    if result.returncode != 0:
        handle_error("Core build failed.")
    print("Core build finished successfully.")

def move_core_binary(destination_dir, binary_name):
    """
    Moves the built core binary from client/build/bin to the given destination directory,
    renaming it as binary_name.
    """
    print(f"Moving core binary to {destination_dir} as {binary_name}...")
    src_name = "r5.exe" if sys.platform.startswith("win") else "r5"
    source = os.path.join("client", "build", "bin", src_name)
    os.makedirs(destination_dir, exist_ok=True)
    dest = os.path.join(destination_dir, binary_name)
    try:
        shutil.move(source, dest)
    except Exception as e:
        handle_error(f"Moving core binary failed: {e}")
    print("Core binary moved successfully.")

# TOOLS AND RELAYER BUILD
# Builds extra tools, such as the CLIWALLET, and the main relayer

def build_relayer():
    print("Building relayer executable...")
    # Prepare the pyinstaller command.
    cmd = [
        'pyinstaller',
        '--onefile',
        '--name', 'relayer',
        '--icon', 'icon.ico',
        'main.py'
    ]
    relayer_dir = os.path.join(os.getcwd(), "relayer")
    print("Executing:", ' '.join(cmd), "in", relayer_dir)
    try:
        subprocess.check_call(cmd, cwd=relayer_dir)
    except subprocess.CalledProcessError as e:
        handle_error(f"Relayer build failed: {e}")
    print("Relayer build completed successfully.")

def build_cliwallet():
    print("Building CLI wallet executable...")
    # Prepare the pyinstaller command.
    cmd = [
        'pyinstaller',
        '--onefile',
        '--name', 'cliwallet',
        '--icon', 'icon.ico',
        'main.py'
    ]
    wallet_dir = os.path.join(os.getcwd(), "cliwallet")
    print("Executing:", ' '.join(cmd), "in", wallet_dir)
    try:
        subprocess.check_call(cmd, cwd=wallet_dir)
    except subprocess.CalledProcessError as e:
        handle_error(f"CLI wallet build failed: {e}")
    print("CLI wallet build completed successfully.")

def move_file(src, dst):
    """
    Moves a file from src to dst.
    """
    print(f"Moving file from {src} to {dst} ...")
    os.makedirs(os.path.dirname(dst), exist_ok=True)
    try:
        shutil.move(src, dst)
    except Exception as e:
        handle_error(f"Error moving file: {e}")
    print("File moved successfully.")

def copy_folder(src, dst):
    """
    Copies the contents of the src folder to the destination folder dst.
    If dst exists, it will be overwritten.
    """
    print(f"Copying from '{src}' to '{dst}'...")
    if not os.path.exists(src):
        print(f"Warning: Source folder {src} does not exist, skipping copy.")
        return
    if os.path.exists(dst):
        shutil.rmtree(dst)
    try:
        shutil.copytree(src, dst)
    except Exception as e:
        handle_error(f"Copying folder '{src}' failed: {e}")
    print(f"Copied '{src}' to '{dst}'.")

# BUILDING LOGIC

def parse_args():
    parser = argparse.ArgumentParser(
        description="R5 Build Script - Build the R5 core and associated components."
    )
    parser.add_argument("--coreonly", action="store_true",
                        help="Build only the core R5 binary (do not perform full build steps).")
    return parser.parse_args()

def main():
    args = parse_args()
    if args.coreonly:
        # Build only the core binary and move it to /build.
        build_core()
        dest_dir = "build"
        binary_name = "r5.exe" if sys.platform.startswith("win") else "r5"
        os.makedirs(dest_dir, exist_ok=True)
        move_core_binary(dest_dir, binary_name)
        sys.exit(0)

    # Full build:
    print("Performing full build...")
    # 1. Build the core binary.
    build_core()
    # 1.a. Move (and rename) the core binary to /build/bin/node (or node.exe)
    dest_core_dir = os.path.join("build", "bin")
    node_binary_name = "node.exe" if sys.platform.startswith("win") else "node"
    move_core_binary(dest_core_dir, node_binary_name)

    # 2. Copy /config into /build/config.
    copy_folder("config", os.path.join("build", "config"))
    # 3. Copy /genesis into /build/json.
    copy_folder("genesis", os.path.join("build", "json"))

    # 4. Build the relayer executable.
    build_relayer()
    # Copy the relayer executable from /relayer/dist to /build/r5 (or r5.exe).
    relayer_bin = "relayer.exe" if sys.platform.startswith("win") else "relayer"
    src_relayer = os.path.join("relayer", "dist", relayer_bin)
    dest_relayer = os.path.join("build", "r5.exe" if sys.platform.startswith("win") else "r5")
    move_file(src_relayer, dest_relayer)

    # 5. Build the CLI wallet executable.
    build_cliwallet()
    # Copy the CLI wallet binary from /cliwallet/dist to /build/bin/cliwallet (or r5wallet.exe).
    wallet_bin = "cliwallet.exe" if sys.platform.startswith("win") else "cliwallet"
    src_wallet = os.path.join("cliwallet", "dist", wallet_bin)
    dest_wallet = os.path.join("build", "bin", wallet_bin)
    move_file(src_wallet, dest_wallet)

    print("\nFull build completed successfully. The folder structure is ready for deployment.")

if __name__ == "__main__":
    main()
