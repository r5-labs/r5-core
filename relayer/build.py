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

import subprocess
import sys

def main():
    cmd = [
        'pyinstaller',
        '--onefile',
        '--name', 'r5-relayer',
        '--icon', 'icon.ico',
        'main.py'
    ]
    print("Building executable with command:", ' '.join(cmd))
    try:
        subprocess.check_call(cmd)
        print("\nBuild successful! The executable can be found in the 'dist' folder.")
    except subprocess.CalledProcessError as e:
        print("Build failed with error:", e)
        sys.exit(e.returncode)

if __name__ == '__main__':
    main()
