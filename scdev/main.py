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
"""
SCdev – Smart Contract Deployer & Interactor for R5

This tool provides an interactive terminal interface to:
  • Navigate the filesystem (cd, ls, mkdir, rm, cp, mv, clear)
  • Change the RPC URL on the fly via the 'rpcurl' command.
  • Compile smart contracts (Solidity and Vyper) using configuration
  • Deploy compiled contracts via an R5 wallet (using a .key file specified in the configuration)
  • Read an ABI file (“readabi”) and list available functions
  • Call contract functions (cf), handling constant (GET) and state‐changing (POST) calls
  • Manage wallets using the 'acc' command (display current, create new, or import)

Configuration is loaded first from a global “scdev.ini” file in the root.
Then, if present, a local “scdev.config” in the current directory overrides those defaults.

Required settings include:
  deployer_wallet    (path to the wallet file, e.g. r5.key)
  rpc_url            (e.g. http://localhost:8545)
  sol_compiler       (e.g. solc)
  vyper_compiler     (e.g. vyper)
  evm_version        (default: berlin)
  optimization       (true/false)
  optimization_runs  (number of runs)

If the global config file does not exist, one is created.
The version is read from a file named "version" in the root.

Commands:
  cd, ls, mkdir, rm, cp, mv, clear, rpcurl, compile, deploy, readabi, cf, acc, help, exit
"""

import os
import sys
import configparser
import subprocess
import shutil
import json
import time
import getpass
import datetime
from web3 import Web3

# ---------------------------
# Global Configuration
# ---------------------------
DEFAULT_GLOBAL_CONFIG = """[SCdev]
deployer_wallet = r5.key
rpc_url = http://localhost:8545
sol_compiler = solc
vyper_compiler = vyper
evm_version = berlin
optimization = false
optimization_runs = 200
"""

GLOBAL_CONFIG_FILE = "scdev.ini"
LOCAL_CONFIG_FILE = "scdev.config"
VERSION_FILE = "version"

def load_global_config():
    config = configparser.ConfigParser()
    if not os.path.exists(GLOBAL_CONFIG_FILE):
        with open(GLOBAL_CONFIG_FILE, "w") as f:
            f.write(DEFAULT_GLOBAL_CONFIG)
        print(f"Created default global config file '{GLOBAL_CONFIG_FILE}'.")
    config.read(GLOBAL_CONFIG_FILE)
    return dict(config["SCdev"])

def load_local_config():
    """Return a dict of settings from a local config file if exists; else {}."""
    local = {}
    if os.path.exists(LOCAL_CONFIG_FILE):
        config = configparser.ConfigParser()
        config.read(LOCAL_CONFIG_FILE)
        if "SCdev" in config:
            local = dict(config["SCdev"])
    return local

def merge_configs(global_cfg, local_cfg):
    """Local settings override global ones."""
    merged = global_cfg.copy()
    merged.update(local_cfg)
    return merged

def load_version():
    if os.path.exists(VERSION_FILE):
        try:
            with open(VERSION_FILE, "r") as f:
                ver = f.read().strip()
                return ver if ver else "version could not be determined"
        except Exception:
            return "version could not be determined"
    return "version could not be determined"

# ---------------------------
# Filesystem Commands
# ---------------------------
def clear_screen():
    os.system('cls' if os.name == 'nt' else 'clear')

def cmd_cd(args):
    if len(args) < 1:
        print("Usage: cd <path>")
        return
    try:
        os.chdir(args[0])
    except Exception as e:
        print(f"Error changing directory: {e}")

def cmd_ls(args):
    for item in os.listdir("."):
        print(item)

def cmd_mkdir(args):
    if len(args) < 1:
        print("Usage: mkdir <dirname>")
        return
    try:
        os.makedirs(args[0], exist_ok=True)
    except Exception as e:
        print(f"Error creating directory: {e}")

def cmd_rm(args):
    if len(args) < 1:
        print("Usage: rm [-rf] <path>")
        return
    force = False
    path = args[0]
    if args[0] in ["-rf", "-r"]:
        force = True
        if len(args) < 2:
            print("Usage: rm -rf <path>")
            return
        path = args[1]
    try:
        if os.path.isdir(path):
            shutil.rmtree(path)
        else:
            os.remove(path)
    except Exception as e:
        print(f"Error removing {path}: {e}")

def cmd_cp(args):
    if len(args) < 2:
        print("Usage: cp <src> <dst>")
        return
    try:
        shutil.copy2(args[0], args[1])
    except Exception as e:
        print(f"Error copying file: {e}")

def cmd_mv(args):
    if len(args) < 2:
        print("Usage: mv <src> <dst>")
        return
    try:
        shutil.move(args[0], args[1])
    except Exception as e:
        print(f"Error moving file: {e}")

def cmd_clear(args):
    clear_screen()

def cmd_rpcurl(args, settings):
    """If a new URL is provided, update the setting; otherwise, display the current RPC URL."""
    if len(args) < 1:
        print(f"Current RPC URL: {settings.get('rpc_url', 'http://localhost:8545')}")
        return
    settings["rpc_url"] = args[0]
    print(f"RPC URL updated to: {args[0]}")

# ---------------------------
# Wallet (Account) Commands – 'acc'
# ---------------------------
from ecdsa import SigningKey, SECP256k1
from cryptography.fernet import Fernet, InvalidToken
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.kdf.pbkdf2 import PBKDF2HMAC
import base64

def derive_key(password: str, salt: bytes) -> bytes:
    kdf = PBKDF2HMAC(
        algorithm=hashes.SHA256(),
        length=32,
        salt=salt,
        iterations=100000,
    )
    return base64.urlsafe_b64encode(kdf.derive(password.encode()))

def encrypt_wallet(wallet: dict, password: str) -> dict:
    salt = os.urandom(16)
    key = derive_key(password, salt)
    f = Fernet(key)
    wallet_json = json.dumps(wallet).encode()
    encrypted_wallet = f.encrypt(wallet_json)
    return {
        "salt": base64.urlsafe_b64encode(salt).decode(),
        "wallet": encrypted_wallet.decode()
    }

def decrypt_wallet(file_data: dict, password: str) -> dict:
    salt = base64.urlsafe_b64decode(file_data["salt"])
    key = derive_key(password, salt)
    f = Fernet(key)
    decrypted = f.decrypt(file_data["wallet"].encode())
    return json.loads(decrypted.decode())

def create_new_wallet() -> dict:
    sk = SigningKey.generate(curve=SECP256k1)
    vk = sk.get_verifying_key()
    wallet = {
        "private_key": sk.to_string().hex(),
        "public_key": vk.to_string().hex()
    }
    return wallet

def create_wallet_with_import() -> dict:
    private_key_input = input("Enter Private Key to import: ").strip()
    try:
        sk = SigningKey.from_string(bytes.fromhex(private_key_input), curve=SECP256k1)
    except Exception as e:
        print("Invalid private key format.")
        return None
    vk = sk.get_verifying_key()
    wallet = {
        "private_key": private_key_input,
        "public_key": vk.to_string().hex()
    }
    return wallet

def display_wallet_info(wallet_filename):
    if not os.path.exists(wallet_filename):
        print(f"No wallet file found at '{wallet_filename}'.")
        return
    try:
        with open(wallet_filename, "r") as f:
            file_data = json.load(f)
    except Exception as e:
        print("Error reading wallet file:", e)
        return
    # Ask user if they wish to display wallet address.
    choice = input("Display wallet address? (y/n): ").strip().lower()
    if choice == "y":
        password = getpass.getpass("Enter wallet password: ")
        try:
            wallet = decrypt_wallet(file_data, password)
            # Derive the address from private key.
            account = Web3().eth.account.from_key(wallet["private_key"])
            print("Wallet Address:", account.address)
        except Exception as e:
            print("Error decrypting wallet:", e)
    else:
        print(f"Wallet file: '{wallet_filename}'")

def cmd_acc(args, settings):
    """
    Account command (acc):
      - If no arguments: display current wallet file (from deployer_wallet setting) and optionally its address.
      - If "new": create a new wallet (optionally with an alias).
      - If "import": import an existing wallet.
    """
    current_wallet = settings.get("deployer_wallet", "r5.key")
    if not args:
        print(f"Current wallet file: '{current_wallet}'")
        display_wallet_info(current_wallet)
        return

    subcmd = args[0].lower()
    if subcmd == "new":
        alias = args[1] if len(args) >= 2 else "r5.key"
        wallet_filename = alias if alias.endswith(".key") else f"{alias}.key"
        if os.path.exists(wallet_filename):
            print(f"Wallet file '{wallet_filename}' already exists. Aborting.")
            return
        wallet = create_new_wallet()
        print("New wallet created.")
    elif subcmd == "import":
        alias = args[1] if len(args) >= 2 else "r5.key"
        wallet_filename = alias if alias.endswith(".key") else f"{alias}.key"
        if os.path.exists(wallet_filename):
            print(f"Wallet file '{wallet_filename}' already exists. Aborting.")
            return
        wallet = create_wallet_with_import()
        if wallet is None:
            print("Wallet import failed.")
            return
    else:
        print("Usage: acc [new [alias]|import [alias]]")
        return

    password = getpass.getpass("Enter a new encryption password: ")
    confirm = getpass.getpass("Confirm password: ")
    if password != confirm:
        print("Passwords do not match. Aborting wallet creation.")
        return
    file_contents = encrypt_wallet(wallet, password)
    try:
        with open(wallet_filename, "w") as f:
            json.dump(file_contents, f)
        # Update the configuration so that deployer_wallet points to the new wallet file.
        settings["deployer_wallet"] = wallet_filename
        print(f"Wallet saved to '{wallet_filename}'.")
    except Exception as e:
        print("Error writing wallet file:", e)

# ---------------------------
# Compilation Functions
# ---------------------------
def compile_solidity(filepath, config):
    solc = config.get("sol_compiler", "solc")
    evm = config.get("evm_version", "berlin")
    optimize = config.get("optimization", "false").lower() == "true"
    runs = config.get("optimization_runs", "200")
    cmd = [solc]
    if optimize:
        cmd.append("--optimize")
        cmd.extend(["--optimize-runs", runs])
    cmd.extend(["--bin", "--abi", "--evm-version", evm, filepath])
    try:
        result = subprocess.run(cmd, capture_output=True, text=True, check=True)
    except subprocess.CalledProcessError as e:
        print("Compilation error:", e.stderr)
        return None, None

    output = result.stdout.splitlines()
    bin_code = ""
    abi_json = ""
    mode = None
    for line in output:
        if "Binary:" in line:
            mode = "bin"
        elif "ABI:" in line:
            mode = "abi"
        else:
            if mode == "bin":
                bin_code += line.strip()
            elif mode == "abi":
                abi_json += line.strip()
    return bin_code, abi_json

def compile_vyper(filepath, config):
    vyper = config.get("vyper_compiler", "vyper")
    cmd = [vyper, "-f", "bytecode,abi", filepath]
    try:
        result = subprocess.run(cmd, capture_output=True, text=True, check=True)
    except subprocess.CalledProcessError as e:
        print("Compilation error:", e.stderr)
        return None, None
    parts = result.stdout.strip().split("\n")
    if len(parts) < 1:
        print("Compilation produced no output.")
        return None, None
    bytecode = parts[0].strip()
    abi = parts[1].strip() if len(parts) > 1 else ""
    return bytecode, abi

def cmd_compile(args, config):
    if len(args) < 1:
        print("Usage: compile <contract_filepath>")
        return
    filepath = args[0]
    if not os.path.exists(filepath):
        print(f"File '{filepath}' does not exist.")
        return
    ext = os.path.splitext(filepath)[1].lower()
    if ext == ".sol":
        bin_code, abi_json = compile_solidity(filepath, config)
    elif ext in [".vy", ".vyper"]:
        bin_code, abi_json = compile_vyper(filepath, config)
    else:
        print("Unsupported contract extension. Use .sol for Solidity or .vy for Vyper.")
        return
    if not bin_code or not abi_json:
        print("Compilation failed.")
        return
    base = os.path.splitext(os.path.basename(filepath))[0]
    out_dir = os.path.dirname(filepath)
    bin_file = os.path.join(out_dir, f"{base}.bin")
    abi_file = os.path.join(out_dir, "abi.json")
    try:
        with open(bin_file, "w") as f:
            f.write(bin_code)
        with open(abi_file, "w") as f:
            f.write(abi_json)
        print(f"Compilation successful. Bytecode written to {bin_file}, ABI written to {abi_file}.")
    except Exception as e:
        print(f"Error writing compiled files: {e}")

# ---------------------------
# Deployment Functions
# ---------------------------
def cmd_deploy(args, config):
    if len(args) < 1:
        print("Usage: deploy <contract_source_filepath>")
        return
    source_file = args[0]
    base, ext = os.path.splitext(source_file)
    bin_file = f"{base}.bin"
    abi_file = os.path.join(os.path.dirname(source_file), "abi.json")
    if not (os.path.exists(bin_file) and os.path.exists(abi_file)):
        print("Contract not compiled. Please run 'compile' first.")
        return
    try:
        with open(bin_file, "r") as f:
            bytecode = f.read().strip()
        with open(abi_file, "r") as f:
            abi = json.load(f)
    except Exception as e:
        print(f"Error reading compiled files: {e}")
        return
    rpc_url = config.get("rpc_url", "http://localhost:8545")
    w3 = Web3(Web3.HTTPProvider(rpc_url))
    if not w3.isConnected():
        print("Error: Unable to connect to RPC at", rpc_url)
        return
    wallet_path = config.get("deployer_wallet", "r5.key")
    if not os.path.exists(wallet_path):
        print(f"Wallet file '{wallet_path}' not found. Please create/import one using the 'acc' command.")
        return
    try:
        with open(wallet_path, "r") as f:
            file_data = json.load(f)
    except Exception as e:
        print("Error reading wallet file:", e)
        return
    password = getpass.getpass("Enter wallet password: ")
    try:
        wallet = decrypt_wallet(file_data, password)
    except Exception as e:
        print("Wallet decryption failed:", e)
        return
    try:
        account = w3.eth.account.from_key(wallet["private_key"])
        deployer_address = account.address
        balance = w3.eth.get_balance(deployer_address)
    except Exception as e:
        print("Error deriving wallet account:", e)
        return
    Contract = w3.eth.contract(abi=abi, bytecode=bytecode)
    try:
        estimated_gas = Contract.constructor().estimateGas({'from': deployer_address})
    except Exception as e:
        print("Error estimating gas:", e)
        return
    gas_price = w3.eth.gas_price
    cost = estimated_gas * gas_price
    if balance < cost:
        print(f"Not enough balance. Estimated cost is {w3.fromWei(cost, 'ether')} R5, but balance is {w3.fromWei(balance, 'ether')} R5.")
        return
    nonce = w3.eth.get_transaction_count(deployer_address)
    tx = Contract.constructor().buildTransaction({
        'from': deployer_address,
        'nonce': nonce,
        'gas': estimated_gas,
        'gasPrice': gas_price
    })
    try:
        signed_tx = w3.eth.account.sign_transaction(tx, wallet["private_key"])
        tx_hash = w3.eth.send_raw_transaction(signed_tx.rawTransaction)
        print("Deploying contract... Transaction hash:", tx_hash.hex())
        receipt = w3.eth.wait_for_transaction_receipt(tx_hash, timeout=300)
        print("Contract successfully deployed at address:", receipt.contractAddress)
        timestamp = datetime.datetime.now().strftime("%d-%m-%Y-%H-%M")
        log_filename = f"{os.path.basename(source_file)}_Deployment-{timestamp}.log"
        with open(log_filename, "w") as logf:
            logf.write(f"Contract deployed at address: {receipt.contractAddress}\n")
            logf.write(f"Transaction hash: {tx_hash.hex()}\n")
            logf.write(f"Gas used: {receipt.gasUsed}\n")
            logf.write(f"Block number: {receipt.blockNumber}\n")
        print(f"Deployment log written to {log_filename}")
    except Exception as e:
        print("Error during contract deployment:", e)

# ---------------------------
# ABI Reading and Function Calling
# ---------------------------
def cmd_readabi(args):
    if len(args) < 1:
        print("Usage: readabi <path_to_abi.json>")
        return
    path = args[0]
    if not os.path.exists(path):
        print(f"ABI file '{path}' not found.")
        return
    try:
        with open(path, "r") as f:
            abi = json.load(f)
    except Exception as e:
        print(f"Error reading ABI file: {e}")
        return
    print("Successfully loaded ABI. Available functions:")
    for item in abi:
        if item.get("type") == "function":
            func_name = item.get("name", "<anonymous>")
            state = item.get("stateMutability", "")
            inputs = item.get("inputs", [])
            input_list = ", ".join([f"{inp['name']} ({inp['type']})" for inp in inputs])
            print(f"{state.upper()} - {func_name}({input_list})")
    print("Use the 'cf' command to call a function.")

def cmd_cf(args, config):
    if len(args) < 2:
        print("Usage: cf <contract_address> <function_name> [parameters...]")
        return
    contract_addr = args[0]
    function_name = args[1]
    parameters = args[2:]
    abi_file = "abi.json"
    if not os.path.exists(abi_file):
        print("ABI file 'abi.json' not found in current directory.")
        return
    try:
        with open(abi_file, "r") as f:
            abi = json.load(f)
    except Exception as e:
        print("Error loading ABI:", e)
        return
    rpc_url = config.get("rpc_url", "http://localhost:8545")
    w3 = Web3(Web3.HTTPProvider(rpc_url))
    contract = w3.eth.contract(address=contract_addr, abi=abi)
    func = None
    for item in abi:
        if item.get("type") == "function" and item.get("name") == function_name:
            func = item
            break
    if func is None:
        print(f"Function '{function_name}' not found in ABI.")
        return
    try:
        state = func.get("stateMutability", "").lower()
        if state in ["view", "pure"]:
            result = contract.functions[function_name](*parameters).call()
            print("Function call result:", result)
        else:
            wallet_path = config.get("deployer_wallet", "r5.key")
            if not os.path.exists(wallet_path):
                print(f"Wallet file '{wallet_path}' not found.")
                return
            try:
                with open(wallet_path, "r") as f:
                    file_data = json.load(f)
            except Exception as e:
                print("Error reading wallet file:", e)
                return
            password = getpass.getpass("Enter wallet password: ")
            try:
                wallet = decrypt_wallet(file_data, password)
            except Exception as e:
                print("Wallet decryption failed:", e)
                return
            account = w3.eth.account.from_key(wallet["private_key"])
            nonce = w3.eth.get_transaction_count(account.address)
            tx = contract.functions[function_name](*parameters).buildTransaction({
                'from': account.address,
                'nonce': nonce,
                'gas': 500000,
                'gasPrice': w3.eth.gas_price
            })
            signed_tx = w3.eth.account.sign_transaction(tx, wallet["private_key"])
            tx_hash = w3.eth.send_raw_transaction(signed_tx.rawTransaction)
            print("Transaction sent. Hash:", tx_hash.hex())
            receipt = w3.eth.wait_for_transaction_receipt(tx_hash, timeout=300)
            print("Transaction mined. Receipt:", receipt)
    except Exception as e:
        print("Error during function call:", e)

# ---------------------------
# Help Command
# ---------------------------
def cmd_help(args):
    print("Available commands:")
    print("  cd <path>              : Change directory")
    print("  ls                     : List files in current directory")
    print("  mkdir <dirname>        : Create directory")
    print("  rm [-rf] <path>        : Remove file or directory")
    print("  cp <src> <dst>         : Copy file")
    print("  mv <src> <dst>         : Move/rename file")
    print("  clear                  : Clear the screen")
    print("  rpcurl [new_url]       : Show or change RPC URL for the session")
    print("  compile <filepath>     : Compile a smart contract (.sol or .vy)")
    print("  deploy <filepath>      : Deploy a compiled contract (requires .bin and abi.json)")
    print("  readabi <path>         : Load an ABI JSON file and list functions")
    print("  cf <addr> <func> [...] : Call a contract function")
    print("  acc [new|import [alias]] : Manage wallet: display current wallet, create new, or import")
    print("  help                   : Show this help message")
    print("  exit                   : Exit SCdev")

# ---------------------------
# Interactive Shell
# ---------------------------
def run_shell(global_settings):
    # Merge with any local overrides.
    local_settings = load_local_config()
    settings = merge_configs(global_settings, local_settings)
    clear_screen()
    # Print title and version.
    print("SCdev – R5 Smart Contract Interface")
    version = load_version()
    print(f"Version: {version}")
    print("")
    prompt = "SCdev # "
    print("Type 'help' for available commands.\n")
    while True:
        try:
            inp = input(prompt)
        except EOFError:
            break
        if not inp.strip():
            continue
        parts = inp.strip().split()
        cmd_name = parts[0].lower()
        args = parts[1:]
        if cmd_name in ["exit", "quit"]:
            print("Exiting SCdev.")
            break
        elif cmd_name == "cd":
            cmd_cd(args)
        elif cmd_name == "ls":
            cmd_ls(args)
        elif cmd_name == "mkdir":
            cmd_mkdir(args)
        elif cmd_name == "rm":
            cmd_rm(args)
        elif cmd_name == "cp":
            cmd_cp(args)
        elif cmd_name == "mv":
            cmd_mv(args)
        elif cmd_name == "clear":
            cmd_clear(args)
        elif cmd_name == "rpcurl":
            cmd_rpcurl(args, settings)
        elif cmd_name == "compile":
            cmd_compile(args, settings)
        elif cmd_name == "deploy":
            cmd_deploy(args, settings)
        elif cmd_name == "readabi":
            cmd_readabi(args)
        elif cmd_name == "cf":
            cmd_cf(args, settings)
        elif cmd_name == "acc":
            cmd_acc(args, settings)
        elif cmd_name == "help":
            cmd_help(args)
        else:
            print("Unknown command. Type 'help' for a list of commands.")

# ---------------------------
# Main Entry Point
# ---------------------------
def main():
    print("Starting SCdev – Smart Contract Deployer")
    global_settings = load_global_config()
    run_shell(global_settings)

if __name__ == "__main__":
    main()
