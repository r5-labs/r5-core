import argparse
import os
import sys
import subprocess

def parse_args():
    parser = argparse.ArgumentParser(
        description="R5 Node Relayer - Simplified entry point for starting an R5 node"
    )
    parser.add_argument(
        "--network",
        choices=["mainnet", "devnet", "testnet", "local"],
        default="mainnet",
        help="Specify the network genesis to use. Defaults to 'mainnet' (which uses built-in settings)."
    )
    # Additional flags can be added here as needed.
    return parser.parse_args()

def get_node_binary():
    # Detect the OS and adjust the binary name accordingly.
    # On Windows, the binary is "node.exe", on other OSes, it's assumed to be "node".
    if sys.platform.startswith("win"):
        return os.path.join("bin", "node.exe")
    else:
        return os.path.join("bin", "node")

def build_command(args):
    node_binary = get_node_binary()
    cmd = [node_binary]
    
    # If the network is not mainnet, add the -genesis flag with the corresponding JSON file.
    # It is assumed that the genesis files are located in the "json" folder.
    if args.network != "mainnet":
        genesis_file = os.path.join("json", f"{args.network}.json")
        cmd.extend(["-genesis", genesis_file])
    
    return cmd

def main():
    args = parse_args()
    cmd = build_command(args)
    
    print("Starting R5 node with command:")
    print(" ".join(cmd))
    
    try:
        subprocess.run(cmd, check=True)
    except subprocess.CalledProcessError as e:
        print(f"Error: Node failed to start with error: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()
