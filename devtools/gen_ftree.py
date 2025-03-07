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
