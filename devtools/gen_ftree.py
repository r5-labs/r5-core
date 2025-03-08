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

import os

def generate_tree(root, prefix=""):
    lines = []
    entries = sorted(os.listdir(root))
    entries_count = len(entries)
    for i, entry in enumerate(entries):
        path = os.path.join(root, entry)
        is_last = (i == entries_count - 1)
        connector = "└── " if is_last else "├── "
        lines.append(prefix + connector + entry)
        if os.path.isdir(path):
            extension = "    " if is_last else "│   "
            lines.extend(generate_tree(path, prefix + extension))
    return lines

def main():
    root_dir = "../"
    lines = [root_dir]
    lines.extend(generate_tree(root_dir))
    with open("folder_tree.txt", "w", encoding="utf-8") as f:
        for line in lines:
            f.write(line + "\n")
    print("Folder tree written to tree.txt")

if __name__ == '__main__':
    main()
