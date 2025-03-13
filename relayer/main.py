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
import configparser
import os
import sys
import subprocess
import shutil
import secrets  # Added for bootnode key generation

DEFAULT_INI = """[R5 Node Relayer]
network = mainnet
rpc = false
mode = default
miner = default
miner_coinbase = default
miner_threads = default
genesis = default
config = default
"""

def ensure_bootnode_key():
    """
    Check for the existence of bootnode.key in the current directory.
    If not found, generate a new 32-byte hex key, save it to bootnode.key,
    and return the key. Otherwise, return the key from the file.
    """
    key_file = "bootnode.key"
    if not os.path.exists(key_file):
        key = secrets.token_hex(32)
        with open(key_file, "w") as f:
            f.write(key)
        print(f"Bootnode key not found. Generated new bootnode key and stored in {key_file}.")
    else:
        with open(key_file, "r") as f:
            key = f.read().strip()
    return key

def get_node_binary():
    """Return the full path to the node binary, adjusted for OS."""
    bin_dir = "bin"
    if os.name == "nt":
        return os.path.join(bin_dir, "node.exe")
    else:
        return os.path.join(bin_dir, "node")

def get_jsconsole_ipc():
    """Return the IPC endpoint for attaching the JS console."""
    if os.name == "nt":
        return r"\\.\pipe\r5"
    else:
        return os.path.join(datadir, "r5")

def get_cliwallet_binary():
    """Return the full path to the CLI Wallet binary, adjusted for OS."""
    bin_dir = "bin"
    if os.name == "nt":
        return os.path.join(bin_dir, "cliwallet.exe")
    else:
        return os.path.join(bin_dir, "cliwallet")

def get_proxy_binary():
    """Return the full path to the Proxy binary, adjusted for OS."""
    bin_dir = "bin"
    if os.name == "nt":
        return os.path.join(bin_dir, "proxy.exe")
    else:
        return os.path.join(bin_dir, "proxy")

def get_console_binary():
    """Return the full path to the R5 console binary, adjusted for OS."""
    bin_dir = "bin"
    if os.name == "nt":
        return os.path.join(bin_dir, "console.exe")
    else:
        return os.path.join(bin_dir, "console")

def load_ini_config(args):
    """
    Read node.ini (creating one with default values if necessary)
    and override args if the value is not "default".
    """
    ini_filename = "node.ini"
    config = configparser.ConfigParser()
    if not os.path.exists(ini_filename):
        with open(ini_filename, "w") as f:
            f.write(DEFAULT_INI)
        print(f"Created default {ini_filename} file.")

    config.read(ini_filename)
    section = "R5 Node Relayer"
    
    # network:
    net_val = config.get(section, "network", fallback="default")
    if net_val.lower() != "default":
        args.network = net_val.lower()
    # rpc:
    rpc_val = config.get(section, "rpc", fallback="default")
    if rpc_val.lower() != "default":
        args.rpc = rpc_val.lower() == "true"
    # mode:
    mode_val = config.get(section, "mode", fallback="default")
    if mode_val.lower() != "default":
        args.mode = mode_val.lower()
    # miner:
    miner_val = config.get(section, "miner", fallback="default")
    if miner_val.lower() == "true":
        miner_list = []
        coinbase_val = config.get(section, "miner_coinbase", fallback="default")
        if coinbase_val.lower() != "default":
            miner_list.append(f"coinbase={coinbase_val}")
        threads_val = config.get(section, "miner_threads", fallback="default")
        if threads_val.lower() != "default":
            miner_list.append(f"threads={threads_val}")
        args.miner = miner_list if miner_list else []
    # config override:
    config_val = config.get(section, "config", fallback="default")
    if config_val.lower() != "default":
        args.config = config_val
    else:
        args.config = None
    # genesis:
    genesis_val = config.get(section, "genesis", fallback="default")
    if genesis_val.lower() != "default":
        args.genesis = genesis_val
    else:
        args.genesis = None

    return args

def build_command(args):
    """
    Build the command to start the node binary based on provided flags.
    """
    cmd = [get_node_binary()]
    
    if args.config:
        config_file = args.config
    else:
        config_file = os.path.join("config", f"{args.network}.config")
    cmd.extend(["-config", config_file])
    
    # Do not pass a -genesis flag here since genesis is already initialized via init.
    
    if args.rpc:
        rpc_flags = [
            "--rpc.allow-unprotected-txs",
            "--graphql",
            "--graphql.corsdomain", "*",
            "--graphql.vhosts", "*",
            "--http.port", "8545",
            "--http",
            "--http.addr", "0.0.0.0",
            "--http.corsdomain", "*",
            "--http.vhosts", "*",
            "--http.api", "eth,net,web3,r5",
            "--ws",
            "--ws.addr", "0.0.0.0",
            "--ws.origins", "*",
            "--ws.api", "eth,net,web3,r5"
        ]
        cmd.extend(rpc_flags)
    
    if args.mode == "archive":
        mode_flags = [
            "--syncmode", "full",
            "--gcmode", "archive",
            "--txlookuplimit=0",
            "--cache.preimages"
        ]
    elif args.mode == "light":
        mode_flags = ["--syncmode", "light"]
    else:
        mode_flags = ["--syncmode", "full"]
    cmd.extend(mode_flags)
    
    if args.miner is not None:
        cmd.append("--mine")
        coinbase = None
        threads = None
        for param in args.miner:
            if param.startswith("coinbase="):
                coinbase = param.split("=", 1)[1]
            elif param.startswith("threads="):
                threads = param.split("=", 1)[1]
        if coinbase is None:
            coinbase = "0x000000000000000000000000000000000000dEaD"
            print("Warning: coinbase not specified, mining rewards will be burned.")
        if threads is None:
            threads = "0"
        cmd.extend(["--miner.etherbase", coinbase, f"--miner.threads={threads}"])
    
    # --- NEW: Bootnode handling ---
    if args.bootnode:
        bootnode_key = ensure_bootnode_key()
        cmd.extend(["-nodekey", bootnode_key])
    
    return cmd

def build_jsconsole_command():
    """Build the command for attaching to the JS console via IPC."""
    return [get_node_binary(), "attach", get_jsconsole_ipc()]

def build_bypass_command(args):
    """
    Build a command for bypass mode.
    All arguments following --bypass are passed directly to the node binary.
    """
    cmd = [get_node_binary()] + args.bypass
    return cmd

def build_cliwallet_command():
    """Build the command to run the CLI Wallet."""
    return [get_cliwallet_binary()]

def build_proxy_command(args):
    """
    Build the command to run the proxy.
    If the optional argument for --proxy is 'gencert', add the --gencert flag.
    """
    cmd = [get_proxy_binary()]
    if args.proxy and args.proxy.lower() == "gencert":
        cmd.append("--gencert")
    return cmd

def build_console_command():
    """Build the command to run the R5 console."""
    return [get_console_binary()]

def get_datadir_from_config(config_file):
    """
    Attempt to extract the datadir from the provided config file.
    If the file contains a TOML [Node] section, look for a line starting with "DataDir".
    Otherwise, fall back to scanning for "--datadir" (not expected in a TOML file).
    """
    try:
        with open(config_file, "r") as f:
            content = f.read()
        if "[Node]" in content:
            for line in content.splitlines():
                if line.strip().startswith("DataDir"):
                    parts = line.split("=")
                    if len(parts) >= 2:
                        return parts[1].strip().strip('"')
            return None
        else:
            tokens = content.split()
            if "--datadir" in tokens:
                idx = tokens.index("--datadir")
                if idx < len(tokens) - 1:
                    return tokens[idx+1]
    except Exception as e:
        print("Warning: Failed to extract datadir from config file:", e)
    return None

def init_genesis(args):
    """
    Initialize genesis using the appropriate genesis file.
    The genesis file is determined solely by the --network flag (or ini file override).
    """
    if args.genesis:
        genesis_file = args.genesis
    else:
        genesis_file = os.path.abspath(os.path.join("genesis", f"{args.network}.json"))
    print(f"Initializing genesis for network '{args.network}' using {genesis_file}...")
    
    if args.config:
        config_file = args.config
    else:
        config_file = os.path.join("config", f"{args.network}.config")
    
    datadir = get_datadir_from_config(config_file)
    if datadir:
        init_cmd = [get_node_binary(), "--datadir", datadir, "init", genesis_file]
    else:
        print("Warning: datadir not found in config file; using default datadir.")
        init_cmd = [get_node_binary(), "init", genesis_file]
    
    try:
        subprocess.run(init_cmd, check=True)
        print("Genesis initialization completed successfully.")
    except subprocess.CalledProcessError as e:
        print(f"Error initializing genesis: {e}")
        sys.exit(1)

def parse_args():
    parser = argparse.ArgumentParser(
        description="R5 Node Relayer - Simplified entry point for starting an R5 node"
    )
    parser.add_argument("--network", choices=["mainnet", "devnet", "testnet", "local"],
                        default="mainnet",
                        help="Specify the network genesis to use. (Default: mainnet uses config/mainnet.config)")
    parser.add_argument("--rpc", action="store_true",
                        help="Enable RPC/HTTP/WS APIs with preset flags.")
    parser.add_argument("--mode", choices=["archive", "full", "light"],
                        default="full",
                        help="Set sync mode: 'archive', 'full', or 'light' (default: full)")
    parser.add_argument("--miner", nargs="*", metavar="PARAM",
                        help=("Enable mining. Accepts optional parameters such as "
                              "'coinbase=ADDRESS' and 'threads=NUM'. "
                              "If coinbase is not specified, it defaults to "
                              "0x000000000000000000000000000000000000dEaD (mining rewards will be burned). "
                              "If threads is not specified, defaults to 0."))
    parser.add_argument("--jsconsole", action="store_true",
                        help=("Start the JS console by attaching via IPC. "
                              "This flag must be used alone."))
    # Advanced flags – these must be used alone.
    parser.add_argument("--bypass", nargs=argparse.REMAINDER,
                        help="Bypass configuration: pass all remaining arguments directly to the node binary. Must be used alone.")
    parser.add_argument("--cliwallet", action="store_true",
                        help="Run the CLI Wallet binary instead of the node binary. Must be used alone.")
    parser.add_argument("--proxy", nargs="?", const="", default=None,
                        help="Run the proxy binary instead of the node binary. Optionally specify 'gencert' to generate self-signed certificates. Must be used alone.")
    parser.add_argument("--r5console", action="store_true",
                        help="Run the R5 console binary instead of the node binary. Must be used alone.")
    # New flag for bootnode handling.
    parser.add_argument("--bootnode", action="store_true",
                        help="If provided, look for a bootnode.key file in the directory; if not found, create one with a bootnode key and pass it to the node with -nodekey.")
    
    parser.set_defaults(genesis=None)
    parser.set_defaults(config=None)
    
    args = parser.parse_args()

    advanced_flags = []
    if args.jsconsole:
        advanced_flags.append("--jsconsole")
    if args.bypass is not None and len(args.bypass) > 0:
        advanced_flags.append("--bypass")
    if args.cliwallet:
        advanced_flags.append("--cliwallet")
    if args.proxy is not None:
        advanced_flags.append("--proxy")
    if args.r5console:
        advanced_flags.append("--r5console")
    if len(advanced_flags) > 1:
        parser.error("Advanced flags (--jsconsole, --bypass, --cliwallet, --proxy, --r5console) must be used alone.")
    if advanced_flags and (args.network != "mainnet" or args.rpc or args.mode != "full" or args.miner is not None):
        parser.error("Advanced flags must be used alone; do not combine with --network, --rpc, --mode, or --miner.")

    # If no advanced flags and no additional command-line arguments, load settings from node.ini.
    if not (args.jsconsole or args.bypass or args.cliwallet or args.proxy or args.r5console) and len(sys.argv) == 1:
        args = load_ini_config(args)
        print("Loaded settings from node.ini:")
        print(f"  network: {args.network}")
        print(f"  rpc: {args.rpc}")
        print(f"  mode: {args.mode}")
        if args.miner is not None:
            print(f"  miner: {args.miner}")
        if args.config:
            print(f"  config file override: {args.config}")
        if args.genesis:
            print(f"  genesis override: {args.genesis}")
    return args

def main():
    args = parse_args()
    
    if args.jsconsole:
        cmd = build_jsconsole_command()
        print("Attaching to JS console with command:")
        print(" ".join(cmd))
        try:
            subprocess.run(cmd, check=True)
        except subprocess.CalledProcessError as e:
            print(f"Error: JS console attach failed: {e}")
            sys.exit(1)
        sys.exit(0)
    
    if args.bypass is not None and len(args.bypass) > 0:
        cmd = build_bypass_command(args)
        print("Running in bypass mode with command:")
        print(" ".join(cmd))
        try:
            subprocess.run(cmd, check=True)
        except subprocess.CalledProcessError as e:
            print(f"Error: Bypass mode failed: {e}")
            sys.exit(1)
        sys.exit(0)
    
    if args.cliwallet:
        cmd = build_cliwallet_command()
        print("Starting CLI Wallet with command:")
        print(" ".join(cmd))
        try:
            subprocess.run(cmd, check=True)
        except subprocess.CalledProcessError as e:
            print(f"Error: CLI Wallet failed to start: {e}")
            sys.exit(1)
        sys.exit(0)
    
    if args.proxy is not None:
        cmd = build_proxy_command(args)
        print("Starting Proxy with command:")
        print(" ".join(cmd))
        try:
            subprocess.run(cmd, check=True)
        except subprocess.CalledProcessError as e:
            print(f"Error: Proxy failed to start: {e}")
            sys.exit(1)
        sys.exit(0)
    
    if args.r5console:
        cmd = build_console_command()
        print("Starting R5 console with command:")
        print(" ".join(cmd))
        try:
            subprocess.run(cmd, check=True)
        except subprocess.CalledProcessError as e:
            print(f"Error: R5 console failed to start: {e}")
            sys.exit(1)
        sys.exit(0)
    
    # For networks other than mainnet, initialize genesis if needed.
    if args.network != "mainnet":
        init_genesis(args)
    
    cmd = build_command(args)
    try:
        subprocess.run(cmd, check=True)
    except subprocess.CalledProcessError as e:
        print(f"Error: Node failed to start with error: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()
