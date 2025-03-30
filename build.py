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
#
# Author: ZNX

import argparse
import os
import sys
import subprocess
import shutil

# --- SUDO check and virtual environment entry on Linux ---
if sys.platform.startswith("linux"):
    # Check for SUDO privileges.
    if os.geteuid() != 0:
        print("This script requires SUDO privileges to run. Please run with sudo.")
        sys.exit(1)
    # If not running from the virtual environment 'r5-venv', re-execute using it.
    if "r5-venv" not in sys.executable:
        venv_python = os.path.join(os.getcwd(), "r5-venv", "bin", "python")
        if not os.path.exists(venv_python):
            print("Virtual environment 'r5-venv' not found. Please run the install script first.")
            sys.exit(1)
        print("Entering virtual environment 'r5-venv'...")
        os.execv(venv_python, [venv_python] + sys.argv)

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
        sys.executable,
        '-m',
        'PyInstaller',
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
        sys.executable,
        '-m',
        'PyInstaller',
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
    
def build_proxy():
    print("Building Proxy executable...")
    # Prepare the pyinstaller command.
    cmd = [
        sys.executable,
        '-m',
        'PyInstaller',
        '--onefile',
        '--name', 'proxy',
        '--icon', 'icon.ico',
        'main.py'
    ]
    proxy_dir = os.path.join(os.getcwd(), "proxy")
    print("Executing:", ' '.join(cmd), "in", proxy_dir)
    try:
        subprocess.check_call(cmd, cwd=proxy_dir)
    except subprocess.CalledProcessError as e:
        handle_error(f"Proxy build failed: {e}")
    print("Proxy build completed successfully.")
    
def build_r5console():
    print("Building R5 Console executable...")
    # Prepare the pyinstaller command.
    cmd = [
        sys.executable,
        '-m',
        'PyInstaller',
        '--onefile',
        '--name', 'console',
        '--icon', 'icon.ico',
        'main.py'
    ]
    r5console_dir = os.path.join(os.getcwd(), "r5console")
    print("Executing:", ' '.join(cmd), "in", r5console_dir)
    try:
        subprocess.check_call(cmd, cwd=r5console_dir)
    except subprocess.CalledProcessError as e:
        handle_error(f"R5 Console build failed: {e}")
    print("R5 Console build completed successfully.")
    
def build_scdev():
    print("Building SCdev executable...")
    # Prepare the pyinstaller command.
    cmd = [
        sys.executable,
        '-m',
        'PyInstaller',
        '--onefile',
        '--name', 'scdev',
        '--icon', 'icon.ico',
        'main.py'
    ]
    scdev_dir = os.path.join(os.getcwd(), "scdev")
    print("Executing:", ' '.join(cmd), "in", scdev_dir)
    try:
        subprocess.check_call(cmd, cwd=scdev_dir)
    except subprocess.CalledProcessError as e:
        handle_error(f"SCdev build failed: {e}")
    print("SCdev build completed successfully.")

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
    
def copy_file(src, dst):
    """
    Copies a single file from src to dst.
    """
    print(f"Copying file from {src} to {dst}...")
    os.makedirs(os.path.dirname(dst), exist_ok=True)
    try:
        shutil.copy2(src, dst)  # Use shutil.copy2 to preserve metadata
    except Exception as e:
        handle_error(f"Error copying file: {e}")
    print("File copied successfully.")

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
    import time
    start = time.time()
    args = parse_args()
    if args.coreonly:
        # Build only the core binary and move it to /build.
        build_core()
        dest_dir = "build"
        binary_name = "r5.exe" if sys.platform.startswith("win") else "r5"
        os.makedirs(dest_dir, exist_ok=True)
        move_core_binary(dest_dir, binary_name)
        end = time.time()
        print(f"\nBuilt R5 in {end - start:.2f} seconds")
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
    # 3. Copy /genesis into /build/genesis.
    copy_folder("genesis", os.path.join("build", "genesis"))

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
    
    # 6. Build the Proxy executable.
    build_proxy()
    # Copy the Proxy binary from /proxy/dist to /build/bin/proxy (or proxy.exe).
    proxy_bin = "proxy.exe" if sys.platform.startswith("win") else "proxy"
    src_proxy = os.path.join("proxy", "dist", proxy_bin)
    dest_proxy = os.path.join("build", "bin", proxy_bin)
    move_file(src_proxy, dest_proxy)
    
    # 7. Build the R5 Console executable.
    build_r5console()
    # Copy the R5 Console binary from /r5console/dist to /build/bin/console (or console.exe).
    r5console_bin = "console.exe" if sys.platform.startswith("win") else "console"
    src_r5console = os.path.join("r5console", "dist", r5console_bin)
    dest_r5console = os.path.join("build", "bin", r5console_bin)
    move_file(src_r5console, dest_r5console)
    
    # 8. Build the SCdev executable.
    build_scdev()
    # Copy the SCdev binary from /scdev/dist to /build/bin/scdev (or scdev.exe).
    scdev_bin = "scdev.exe" if sys.platform.startswith("win") else "scdev"
    src_scdev = os.path.join("scdev", "dist", scdev_bin)
    dest_scdev = os.path.join("build", "bin", scdev_bin)
    move_file(src_scdev, dest_scdev)
    # Copy the SCdev version file from /scdev/dist to /build/bin/scdev.version.
    scdev_version = "scdev.version"
    src_scdev_version = os.path.join("scdev", scdev_version)
    dest_scdev_version = os.path.join("build", "bin", scdev_version)
    copy_file(src_scdev_version, dest_scdev_version)

    print("\nFull build completed successfully. The folder structure is ready for deployment.")
    end = time.time()
    print(f"R5 Build finished in {end - start:.2f} seconds")

if __name__ == "__main__":
    main()
