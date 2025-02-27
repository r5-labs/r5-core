import subprocess
import sys

def main():
    cmd = [
        'pyinstaller',
        '--onefile',
        '--name', 'r5wallet',
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
