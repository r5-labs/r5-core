# Build R5 Miner

After installing all dependencies using `requirements.txt`, you can build the R5 with PyInstaller:

```bash
pyinstaller --onefile --name r5miner --hidden-import ethash_r5 --icon icon.ico main.py
```

Or

```bash
python3 -m PyInstaller --onefile --name r5miner --hidden-import ethash_r5 --icon icon.ico main.py
```