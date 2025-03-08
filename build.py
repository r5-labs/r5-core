import subprocess
import sys
import os
import shutil

def handle_error():
    print("Error: Script failed to build R5.")
    sys.exit(1)

def clean_cache():
    print("Cleaning build cache...")
    subprocess.run(["go", "clean", "-cache"], cwd="client", check=True)

def build_r5():
    print("Building R5. Please wait...")
    clean_cache()
    result = subprocess.run(["go", "run", "build/ci.go", "install", "./cmd/r5"], cwd="client", check=False)
    if result.returncode != 0:
        handle_error()
    print("Build finished successfully.")

def move_r5():
    print("Moving binary to build folder...")
    # Use .exe on Windows
    binary_name = "r5.exe" if sys.platform.startswith("win") else "r5"
    source = os.path.join("client", "build", "bin", binary_name)
    dest_dir = "build"
    dest = os.path.join(dest_dir, binary_name)
    os.makedirs(dest_dir, exist_ok=True)
    try:
        shutil.move(source, dest)
    except Exception as e:
        print(f"Error moving r5 binary: {e}")
        handle_error()
    print("Binary moved successfully.")
    print("Your R5 binary was built and placed inside the '/build' directory.")

if __name__ == "__main__":
    build_r5()
    move_r5()
