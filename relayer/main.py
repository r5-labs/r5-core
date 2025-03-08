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

DEFAULT_INI = """[R5 Node Relayer]
network=mainnet
rpc=false
mode=default
miner=default
miner.coinbase=default
miner.threads=default
genesis=default
config=default
"""

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
        return "r5.ipc"

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
    
    # For each value, if its value is not "default", update the args.
    # network:
    net_val = config.get(section, "network", fallback="default")
    if net_val.lower() != "default":
        args.network = net_val.lower()
    # rpc:
    rpc_val = config.get(section, "rpc", fallback="default")
    if rpc_val.lower() != "default":
        args.rpc = rpc_val.lower() == "true"
    # mode: note our argparse default is "full", so if the INI value is not "default",
    # we expect one of archive, full, or light.
    mode_val = config.get(section, "mode", fallback="default")
    if mode_val.lower() != "default":
        args.mode = mode_val.lower()
    # miner:
    miner_val = config.get(section, "miner", fallback="default")
    # If miner is "true" (or "yes"), then we want to enable mining. Otherwise leave as None.
    if miner_val.lower() == "true":
        # In our argparse, miner is a list if provided.
        miner_list = []
        coinbase_val = config.get(section, "miner.coinbase", fallback="default")
        if coinbase_val.lower() != "default":
            miner_list.append(f"coinbase={coinbase_val}")
        # If coinbase is default, we leave it unset (so our code will default to burning)
        threads_val = config.get(section, "miner.threads", fallback="default")
        if threads_val.lower() != "default":
            miner_list.append(f"threads={threads_val}")
        # If miner_list is empty, we still want to signal mining enabled
        args.miner = miner_list if miner_list else []
    # Else if miner is "false" or "default", we leave args.miner as None.
    
    # For the config file override:
    config_val = config.get(section, "config", fallback="default")
    if config_val.lower() != "default":
        args.config_override = config_val  # add a new attribute for later use
    else:
        args.config_override = None

    # The genesis field we do not use here since for mainnet we ignore it.
    # (You could add additional logic here if your node binary supported a genesis flag.)
    return args

def build_command(args):
    """
    Build the command to start the node binary based on provided flags.
    All networks (even mainnet) now supply a config file.
    """
    cmd = [get_node_binary()]
    
    # Use config_override if provided; otherwise use default config file based on network.
    if args.config_override:
        config_file = args.config_override
    else:
        config_file = os.path.join("config", f"{args.network}.config")
    cmd.extend(["-config", config_file])
    
    # Append RPC flags if requested.
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
    
    # Append sync mode flags.
    if args.mode == "archive":
        mode_flags = [
            "--syncmode", "full",
            "--gcmode", "archive",
            "--txlookuplimit=0",
            "--cache.preimages"
        ]
    elif args.mode == "light":
        mode_flags = ["--syncmode", "light"]
    else:  # full (or default)
        mode_flags = ["--syncmode", "full"]
    cmd.extend(mode_flags)
    
    # Append miner flags if provided.
    if args.miner is not None:
        # Enable mining with --mine (note: node binary expects --mine, not --miner)
        cmd.append("--mine")
        coinbase = None
        threads = None
        # Look through the list for parameters like coinbase=... and threads=...
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
    
    return cmd

def build_jsconsole_command():
    """Build the command for attaching to the JS console via IPC."""
    return [get_node_binary(), "attach", get_jsconsole_ipc()]

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
    # We will later add a hidden attribute for a config override (from the INI file)
    parser.set_defaults(config_override=None)
    args = parser.parse_args()

    # Enforce that if --jsconsole is provided, no other non-default flags are used.
    if args.jsconsole:
        other_used = []
        if args.network != "mainnet":
            other_used.append("--network")
        if args.rpc:
            other_used.append("--rpc")
        if args.mode != "full":
            other_used.append("--mode")
        if args.miner is not None:
            other_used.append("--miner")
        if other_used:
            parser.error("--jsconsole must be used alone.")
    return args

def main():
    # First, parse command-line arguments.
    args = parse_args()
    
    # If --help or --jsconsole is used, ignore the INI file.
    if not args.jsconsole and len(sys.argv) == 1:
        # No flags provided; try to load settings from node.ini.
        args = load_ini_config(args)
        print("Loaded settings from node.ini:")
        print(f"  network: {args.network}")
        print(f"  rpc: {args.rpc}")
        print(f"  mode: {args.mode}")
        if args.miner is not None:
            print(f"  miner: {args.miner}")
        if args.config_override:
            print(f"  config file override: {args.config_override}")
    
    # If jsconsole is requested, run that command and exit.
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
    
    # Build the command to run the node.
    cmd = build_command(args)
    # Uncomment for verbose and debugging
    # print("Starting R5 node with command:")
    # print(" ".join(cmd))
    try:
        subprocess.run(cmd, check=True)
    except subprocess.CalledProcessError as e:
        print(f"Error: Node failed to start with error: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()
