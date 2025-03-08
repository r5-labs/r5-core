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
R5 Console

Welcome to the R5 Console – an interactive, user-friendly environment for querying your R5 node via JSON‑RPC.
By default, the console connects to http://localhost:8545. Use the --rpcurl flag to override the default URL.
Commands can be entered as JSON or in a simplified format (e.g. "r5_getBalance 0xADDRESS").
Type "exit" to quit.
"""

import argparse
import json
import requests  # type: ignore
import shlex
import os

# Setup command history via readline (if available)
HISTORY_FILE = "history.log"
HISTORY_LIMIT = 100

try:
    import readline
    if os.path.exists(HISTORY_FILE):
        try:
            readline.read_history_file(HISTORY_FILE)
        except Exception as e:
            print("Warning: Could not load history file:", e)
    readline.set_history_length(HISTORY_LIMIT)
except ImportError:
    readline = None

def parse_args():
    parser = argparse.ArgumentParser(
        description="R5 Console – Interactive JSON‑RPC client for R5 Node"
    )
    parser.add_argument("--rpcurl", type=str, default="http://localhost:8545",
                        help="Override the default RPC URL (default: http://localhost:8545)")
    return parser.parse_args()

def try_convert_hex(obj):
    """
    Recursively walk through obj.
    If any value is a string starting with "0x" and can be interpreted as a hex number,
    convert it to its decimal string representation.
    """
    if isinstance(obj, dict):
        new_obj = {}
        for k, v in obj.items():
            new_obj[k] = try_convert_hex(v)
        return new_obj
    elif isinstance(obj, list):
        return [try_convert_hex(item) for item in obj]
    elif isinstance(obj, str) and obj.startswith("0x"):
        try:
            # Attempt to convert hex string to integer.
            # int() handles both uppercase and lowercase letters.
            dec_val = int(obj, 16)
            return str(dec_val)
        except Exception:
            return obj
    else:
        return obj

def main():
    args = parse_args()
    
    # Clear the screen at startup.
    if os.name == "nt":
        os.system("cls")
    else:
        os.system("clear")
    
    print("Welcome to the R5 Console")
    print("Connected to RPC URL:", args.rpcurl)
    print("-" * 95)
    print("Enter your JSON‑RPC command in one of two formats:")
    print("")
    print("  1. Full JSON queries:")
    print("     (e.g.: {\"jsonrpc\": \"2.0\", \"method\": \"r5_getBalance\", \"params\": [\"0x123...\"], \"id\": 1})")
    print("  2. Simplified queries:")
    print("     Method followed by parameters (e.g.: r5_getBalance 0x123... latest [--trydec])")
    print("")
    print("You can add the token --trydec to the end of your simplified query to try to convert HEX to DEC")
    print("in the response. This is a \"best effort\" approach, and won't work with complex responses.")
    print("-" * 95)
    print("Type 'exit' to quit or 'clear' to clear the screen.\n")
    
    while True:
        try:
            line = input("# ").strip()
        except (EOFError, KeyboardInterrupt):
            print("\nExiting R5 Console.")
            break
        
        if not line:
            continue
        
        if line.lower() == "exit":
            print("Exiting R5 Console.")
            break
        
        if line.lower() == "clear":
            if os.name == "nt":
                os.system("cls")
            else:
                os.system("clear")
            continue

        # Add non-empty command to history if readline is available.
        if readline:
            readline.add_history(line)
            
         # Flag to indicate whether to try to convert hex to decimal.
        try_dec = False
        
        # Try to interpret the line as JSON.
        try:
            request_obj = json.loads(line)
        except json.JSONDecodeError:
            # Fallback: use shlex to split the line.
            try:
                tokens = shlex.split(line)
                if not tokens:
                    continue
                # Remove any occurrence of "--trydec" from the tokens.
                filtered_tokens = []
                for token in tokens:
                    if token.lower() == "--trydec":
                        try_dec = True
                    else:
                        filtered_tokens.append(token)
                if not filtered_tokens:
                    continue
                method = filtered_tokens[0]
                params = []
                for token in filtered_tokens[1:]:
                    try:
                        params.append(int(token))
                    except ValueError:
                        try:
                            params.append(float(token))
                        except ValueError:
                            params.append(token)
                request_obj = {
                    "jsonrpc": "2.0",
                    "method": method,
                    "params": params,
                    "id": 1
                }
            except Exception as e:
                print("Error parsing command:", e)
                continue
            
        # Send the JSON-RPC request.
        try:
            response = requests.post(args.rpcurl, json=request_obj)
            if response.status_code == 200:
                try:
                    resp_json = response.json()
                    # If try_dec flag is set, attempt to convert any HEX strings in the result.
                    if try_dec:
                        resp_json = try_convert_hex(resp_json)
                    print(json.dumps(resp_json, indent=2))
                except json.JSONDecodeError:
                    print("Received non-JSON response:")
                    print(response.text)
            else:
                print(f"HTTP Error {response.status_code}:")
                print(response.text)
        except Exception as e:
            print("Error sending request:", e)
    
    # On exit, if readline is available, write history to file.
    if readline:
        try:
            readline.write_history_file(HISTORY_FILE)
        except Exception as e:
            print("Warning: Could not save command history:", e)

if __name__ == "__main__":
    main()
