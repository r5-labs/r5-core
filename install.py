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
# This script installs the necessary system dependencies (e.g. GCC, Golang 1.19)
# and then installs the Python dependencies (via our consolidated setup.py).
# Administrative access may be required.
# Compatible with Windows, Linux, and macOS.

import sys
import subprocess
import os

def run_command(command, shell=False):
    try:
        subprocess.run(command, check=True, shell=shell)
        return True
    except subprocess.CalledProcessError:
        return False

def install_homebrew():
    print("Installing Homebrew...")
    return run_command(
        ["/bin/bash", "-c", "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"],
        shell=False
    )

def install_chocolatey():
    print("Installing Chocolatey...")
    # Run the Chocolatey install command in PowerShell.
    return run_command(
        ['powershell', 'Set-ExecutionPolicy', 'Bypass', '-Scope', 'Process', '-Force',
         ';', 'iwr', 'https://chocolatey.org/install.ps1', '-UseBasicParsing', '|', 'iex'],
        shell=True
    )

def check_or_install_package_manager():
    if sys.platform.startswith('darwin'):
        # Check if Homebrew is installed.
        if not run_command(['which', 'brew']):
            if not install_homebrew():
                print("Failed to install Homebrew.")
                sys.exit(1)
        return 'brew'
    elif sys.platform.startswith('win'):
        # Check if Chocolatey is installed.
        if not run_command(['choco', '--version']):
            if not install_chocolatey():
                print("Failed to install Chocolatey.")
                sys.exit(1)
        return 'choco'
    return None

def install_package(package, installer):
    print(f"Attempting to install {package}...")
    if installer == 'brew':
        # For macOS with Homebrew: install Go 1.19 using go@1.19 formula.
        if package == 'go':
            cmd = [installer, 'install', 'go@1.19']
        else:
            cmd = [installer, 'install', package]
    elif installer == 'choco':
        # For Windows with Chocolatey, specify version for golang.
        if package == 'golang':
            # Adjust the version string as needed.
            cmd = [installer, 'install', 'golang', '--version=1.19.4', '-y']
        elif package == 'mingw-w64':
            # Use the mingw-w64 package for GCC on Windows.
            cmd = [installer, 'install', 'mingw-w64', '-y']
        else:
            cmd = [installer, 'install', package, '-y']
    elif sys.platform.startswith('linux'):
        # For Linux, we attempt to use snap for Go 1.19.
        if package == 'golang':
            cmd = ['sudo', 'snap', 'install', 'go', '--channel=1.19/stable']
        else:
            cmd = ['sudo', 'apt-get', 'install', '-y', package]
    else:
        cmd = [installer, 'install', package]
    
    if not run_command(cmd):
        print(f"Failed to install {package}. Please check your installation settings and permissions.")
        sys.exit(1)

def install_system_dependencies():
    if sys.platform.startswith('linux'):
        # For Linux, install Go via snap and gcc via apt-get.
        install_package('golang', 'snap')
        install_package('gcc', 'apt-get')
    elif sys.platform.startswith('darwin'):
        installer = check_or_install_package_manager()
        # For macOS, install Go 1.19 and gcc.
        install_package('go', installer)
        install_package('gcc', installer)
    elif sys.platform.startswith('win'):
        installer = check_or_install_package_manager()
        # For Windows, install golang (specified with version) and mingw-w64 (which provides gcc).
        install_package('golang', installer)
        install_package('mingw-w64', installer)
    else:
        print("Unsupported operating system.")
        sys.exit(1)

def install_python_dependencies():
    """
    Run pip install . to install the Python dependencies (and create console scripts)
    from the root setup.py.
    """
    print("Installing Python dependencies from setup.py...")
    try:
        # Using sys.executable to get the current Python interpreter.
        subprocess.check_call([sys.executable, "-m", "pip", "install", "."])
        print("Python dependencies installed successfully.")
    except subprocess.CalledProcessError as e:
        print(f"Failed to install Python dependencies: {e}")
        sys.exit(e.returncode)

def main():
    install_system_dependencies()
    install_python_dependencies()
    print("All dependencies installed successfully.")

if __name__ == "__main__":
    main()
