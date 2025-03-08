#! /usr/bin/env python3
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

import os
import sys
import json
import base64
import getpass
import time
from ecdsa import SigningKey, SECP256k1
from cryptography.fernet import Fernet, InvalidToken
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.kdf.pbkdf2 import PBKDF2HMAC
from web3 import Web3
import platform

def set_window_title(title):
    if os.name == 'nt':  # Windows
        os.system(f"title {title}")
    else:  # Most UNIX terminals support this ANSI escape sequence:
        sys.stdout.write(f"\x1b]2;{title}\x07")

set_window_title("R5 CLI Wallet")

WALLET_FILENAME = "r5.key"
SETTINGS_FILENAME = "settings.r5w"
DEFAULT_RPC_ADDRESS = "http://localhost:8545"
DEFAULT_QUERY_INTERVAL = 60

# -------------------------------
# Helper functions for UI
# -------------------------------
def clear_screen():
    os.system('cls' if os.name == 'nt' else 'clear')

def print_header():
    # Read version from file "version" if it exists; otherwise, default to "1.0.0"
    version = "1.0.0"
    if os.path.exists("version"):
        try:
            with open("version", "r") as f:
                version = f.read().strip()
        except Exception:
            pass
    print("")
    print("                         ┌ R5 CLI WALLET ┐")
    print("                         └     v" + version + "    ┘")
    print("")
    print("")

def pause():
    # Waits for input but auto-continues after 5 seconds.
    _ = timed_input("Refreshing in 5 seconds. Press any key to refresh now.", 5)

def manual_pause():
    # Waits until the user presses Enter.
    input("Press any key to continue.")

def timed_input(prompt, timeout):
    """Wait for user input for timeout seconds. If none is given, return empty string."""
    print(prompt, end="", flush=True)
    result = ""
    start_time = time.time()
    if os.name == "nt":
        import msvcrt
        while True:
            if msvcrt.kbhit():
                ch = msvcrt.getwche()
                if ch in ("\r", "\n"):
                    print()
                    break
                result += ch
            if time.time() - start_time > timeout:
                break
            time.sleep(0.1)
    else:
        import sys, select
        rlist, _, _ = select.select([sys.stdin], [], [], timeout)
        if rlist:
            result = sys.stdin.readline().rstrip('\n')
    return result.strip()

# -------------------------------
# Settings File Functions
# -------------------------------
def load_settings():
    settings = {}
    if os.path.exists(SETTINGS_FILENAME):
        try:
            with open(SETTINGS_FILENAME, "r") as f:
                lines = f.readlines()
            # Check for a proper header and footer
            if not lines or not lines[0].strip().startswith("# r5w") or not lines[-1].strip().startswith("#r5w"):
                raise ValueError("Header/footer missing")
            for line in lines[1:-1]:
                line = line.strip()
                if line and "=" in line:
                    key, value = line.split("=", 1)
                    settings[key.strip()] = value.strip()
        except Exception as e:
            print("Warning: Settings file not detected or corrupted. Creating new settings file...")
            pause()
            settings = create_default_settings()
    else:
        print("Warning: Settings file not detected or corrupted. Creating new settings file...")
        pause()
        settings = create_default_settings()
    if "rpc_address" not in settings:
        print("Warning: Required setting 'rpc_address' missing. Creating new settings file with defaults...")
        pause()
        settings = create_default_settings()
    # Ensure query_interval exists (default to DEFAULT_QUERY_INTERVAL)
    if "query_interval" not in settings:
        settings["query_interval"] = str(DEFAULT_QUERY_INTERVAL)
        with open(SETTINGS_FILENAME, "w") as f:
            f.write("# r5w-start\n")
            for key, value in settings.items():
                f.write(f"{key} = {value}\n")
            f.write("# r5w-end\n")
    return settings

def create_default_settings():
    settings = {"rpc_address": DEFAULT_RPC_ADDRESS, "query_interval": str(DEFAULT_QUERY_INTERVAL)}
    with open(SETTINGS_FILENAME, "w") as f:
        f.write("# r5w-start\n")
        for key, value in settings.items():
            f.write(f"{key} = {value}\n")
        f.write("# r5w-end\n")
    return settings

# -------------------------------
# Wallet Encryption / Decryption
# -------------------------------
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
    wallet = json.loads(decrypted.decode())
    return wallet

def prompt_for_password() -> str:
    while True:
        pwd1 = getpass.getpass("Create Encryption Password: ")
        pwd2 = getpass.getpass("Confirm Encryption Password: ")
        if pwd1 != pwd2:
            print("Passwords don't match. Try again.")
            pause()
        else:
            return pwd1

# -------------------------------
# Wallet Creation / Import
# -------------------------------
def create_wallet_with_import() -> dict:
    private_key_input = input("Import Private Key: ").strip()
    try:
        sk = SigningKey.from_string(bytes.fromhex(private_key_input), curve=SECP256k1)
    except Exception as e:
        print("Invalid private key format.")
        pause()
        return None
    vk = sk.get_verifying_key()
    wallet = {
        "private_key": private_key_input,
        "public_key": vk.to_string().hex()
    }
    return wallet

def create_new_wallet() -> dict:
    sk = SigningKey.generate(curve=SECP256k1)
    vk = sk.get_verifying_key()
    wallet = {
        "private_key": sk.to_string().hex(),
        "public_key": vk.to_string().hex()
    }
    return wallet

def wallet_setup():
    if not os.path.exists(WALLET_FILENAME):
        print("└→ No existing wallet detected!")
        print("-" * 68)
        choice = input("Do you want to import a wallet with a private key? (y/n): ").strip().lower()
        if choice == 'y':
            wallet = create_wallet_with_import()
            if wallet is None:
                print("Wallet import failed.")
                pause()
                sys.exit(1)
        else:
            clear_screen()
            print("\n└→ CREATE NEW R5 WALLET")
            print("-" * 68)
            wallet = create_new_wallet()
        password = prompt_for_password()
        file_contents = encrypt_wallet(wallet, password)
        with open(WALLET_FILENAME, "w") as f:
            json.dump(file_contents, f)
        print("Wallet file created.")
        return wallet
    else:
        try:
            with open(WALLET_FILENAME, "r") as f:
                file_data = json.load(f)
        except Exception as e:
            print("Wallet file is corrupt.")
            pause()
            file_data = None

        if file_data:
            password = getpass.getpass("Encryption Password: ")
            try:
                wallet = decrypt_wallet(file_data, password)
                return wallet
            except (InvalidToken, Exception) as e:
                print("Incorrect password or corrupt wallet.")
        choice = input("Do you want to reimport the wallet using its private key? (y/n): ").strip().lower()
        if choice == 'y':
            wallet = create_wallet_with_import()
            if wallet is None:
                print("Wallet import failed.")
                pause()
                sys.exit(1)
            password = prompt_for_password()
            file_contents = encrypt_wallet(wallet, password)
            with open(WALLET_FILENAME, "w") as f:
                json.dump(file_contents, f)
            return wallet
        else:
            choice2 = input("Do you want to create a new wallet? (y/n): ").strip().lower()
            if choice2 == 'y':
                wallet = create_new_wallet()
                password = prompt_for_password()
                file_contents = encrypt_wallet(wallet, password)
                with open(WALLET_FILENAME, "w") as f:
                    json.dump(file_contents, f)
                return wallet
            else:
                print("Could not initiate R5 CLI Wallet. Exiting.")
                pause()
                sys.exit(1)

# -------------------------------
# Web3 / JSON-RPC Functions
# -------------------------------
def fetch_block_height(w3: Web3) -> int:
    try:
        return w3.eth.block_number
    except Exception as e:
        print("Error fetching block height:", e)
        pause()
        return 0

def get_wallet_address(wallet: dict, w3: Web3) -> str:
    try:
        account = w3.eth.account.from_key(wallet["private_key"])
        return account.address
    except Exception as e:
        print("Error deriving wallet address:", e)
        pause()
        return "Unknown"

def fetch_balance(w3: Web3, wallet: dict):
    address = get_wallet_address(wallet, w3)
    try:
        balance_wei = w3.eth.get_balance(address)
        balance = w3.from_wei(balance_wei, 'ether')
        return float(balance)
    except Exception as e:
        print("Error fetching balance:", e)
        pause()
        return 0.0

def fetch_history(w3: Web3, wallet: dict, block_range: int = 1080):
    address = get_wallet_address(wallet, w3).lower()
    current_block = fetch_block_height(w3)
    start_block = max(0, current_block - block_range)
    transactions = []
    # Note: Removed printing here for a cleaner final TX history screen.
    for blk in range(start_block, current_block + 1):
        try:
            block = w3.eth.get_block(blk, full_transactions=True)
            for tx in block.transactions:
                if tx['from'].lower() == address or (tx.to and tx.to.lower() == address):
                    tx_info = {
                        "blockNumber": tx.blockNumber,
                        "from": tx['from'],
                        "to": tx.to,
                        "value": w3.from_wei(tx.value, 'ether'),
                        "hash": tx.hash.hex()
                    }
                    transactions.append(tx_info)
        except Exception:
            continue
    return transactions, block_range

def estimate_gas(w3: Web3, wallet: dict, destination: str, amount_wei: int):
    sender = get_wallet_address(wallet, w3)
    tx = {
        "from": sender,
        "to": destination,
        "value": amount_wei
    }
    try:
        gas_estimate = w3.eth.estimate_gas(tx)
        return gas_estimate
    except Exception as e:
        print("Error estimating gas:", e)
        pause()
        return 21000

def send_tx(w3: Web3, wallet: dict):
    clear_screen()
    print_header()
    sender = get_wallet_address(wallet, w3)
    print("\n└→ SEND TRANSACTION")
    print("-" * 68)
    print("From:", sender)
    print("-" * 68)
    destination = input("To: ").strip()
    print("-" * 68)
    amount_str = input("Amount: ").strip()
    print("-" * 68)
    if amount_str == "":
        amount_float = 0.0
    else:
        try:
            amount_float = float(amount_str)
        except ValueError:
            print("Invalid amount format.")
            pause()
            return
    amount_wei = w3.to_wei(amount_float, 'ether')
    print("Transaction Gas Settings (leave blank to calculate automatically):")
    default_gas = estimate_gas(w3, wallet, destination, amount_wei)
    gas_input = input(f"\nMax. Fee [Estimated: {default_gas}u]: ").strip()
    if gas_input == "":
        gas_limit = default_gas
    else:
        try:
            gas_limit = int(gas_input)
        except ValueError:
            print("Invalid max fee. Using default.")
            gas_limit = default_gas
    # New field for gas price
    default_gas_price = w3.eth.gas_price
    gas_price_input = input(f"Gas Price [Estimated: {w3.from_wei(default_gas_price, 'gwei'):.0f}]: ").strip()
    if gas_price_input == "":
        gas_price = default_gas_price
    else:
        try:
            gas_price = w3.to_wei(float(gas_price_input), 'gwei')
        except Exception as e:
            print("Invalid gas price. Using estimated value.")
            gas_price = default_gas_price
    print("-" * 68)
    confirm = input("Do you confirm the transaction above? (y/n) ").strip().lower()
    print("-" * 68)
    if confirm != 'y':
        print("Transaction cancelled.")
        pause()
        return
    print("→ Sending transaction to blockchain", end="", flush=True)
    for _ in range(3):
        time.sleep(0.5)
        print(".", end="", flush=True)
    print()
    try:
        nonce = w3.eth.get_transaction_count(sender)
    except Exception as e:
        print("Error fetching nonce:", e)
        pause()
        return
    tx = {
        "nonce": nonce,
        "to": destination,
        "value": amount_wei,
        "gas": gas_limit,
        "gasPrice": gas_price,
    }
    try:
        signed_tx = w3.eth.account.sign_transaction(tx, wallet["private_key"])
    except Exception as e:
        print("Error signing transaction:", e)
        pause()
        return
    try:
        tx_hash = w3.eth.send_raw_transaction(signed_tx.raw_transaction)
        print("→ Processing transaction", end="", flush=True)
        for _ in range(3):
            time.sleep(0.5)
            print(".", end="", flush=True)
        print()
        receipt = w3.eth.wait_for_transaction_receipt(tx_hash, timeout=1200)
        print("→ Transaction successfully mined!")
        print("\n└→ Your Receipt:")
        print("-" * 68)
        print("›", Web3.to_hex(tx_hash))
        print("-" * 68)
        manual_pause()
    except Exception as e:
        print("Error sending transaction:", e)
        manual_pause()

# -------------------------------
# Menu Functions
# -------------------------------
def expose_private_key(wallet: dict):
    # Get the password without showing the subsequent UI
    entered_password = getpass.getpass("Enter encryption password to expose private key: ")
    try:
        with open(WALLET_FILENAME, "r") as f:
            file_data = json.load(f)
        _ = decrypt_wallet(file_data, entered_password)
        # Once verified, clear the screen and show only the key
        clear_screen()
        print_header()
        print("\n└→ PRIVATE KEY")
        print("-" * 68)
        print("››", wallet["private_key"])
        print("-" * 68)
    except Exception:
        print("Incorrect password or error decrypting wallet. Returning to main menu.")
    input("Press any key to return to the main menu.")

def reset_wallet():
    clear_screen()
    print_header()
    print("\n└→ RESET WALLET")
    print("-" * 68)
    print("THIS WILL DELETE THE EXISTING WALLET FROM THE SYSTEM, MAKING IT")
    print("UNACCESSIBLE FOREVER! YOU CAN'T UNDO THIS!")
    print("-" * 68)
    confirm = input("ARE YOU SURE YOU WANT TO CONTINUE? (y/n): ").strip().lower()
    print("-" * 68)
    if confirm != 'y':
        print("Wallet reset cancelled.")
        pause()
        return
    password = getpass.getpass("Enter encryption password to confirm wallet reset: ")
    try:
        with open(WALLET_FILENAME, "r") as f:
            file_data = json.load(f)
        _ = decrypt_wallet(file_data, password)
        os.remove(WALLET_FILENAME)
        print("Wallet has been reset. Please restart the application to create or import a new wallet.")
        pause()
        sys.exit(0)
    except Exception:
        print("Incorrect password or error decrypting wallet. Wallet reset aborted.")
        pause()

def show_tx_history(w3: Web3, wallet: dict):
    # Show a temporary message, then clear it when done.
    clear_screen()
    print_header()
    print("\nFetching transaction history. Please wait...")
    transactions, block_range = fetch_history(w3, wallet)
    clear_screen()
    print_header()
    print(f"\n└→ TRANSACTION HISTORY (PAST {block_range} BLOCKS):")
    print("-" * 68)
    if not transactions:
        print("No transactions found in the specified block range.")
    else:
        for tx in transactions:
            print(f"Block: {tx['blockNumber']}")
            print(f"Tx: {tx['hash']}")
            print(f"From: {tx['from']}")
            print(f"To: {tx['to']}")
            print(f"Amount: R5 {tx['value']:.4f}")
            print("-" * 68)
    input("Press any key to return to the main menu.")

def run_main_menu(w3: Web3, wallet: dict, query_interval: int):
    while True:
        clear_screen()
        print_header()
        block_height = fetch_block_height(w3)
        balance = fetch_balance(w3, wallet)
        address = get_wallet_address(wallet, w3)
        print("Address:", address)
        print("RPC URL:", w3.provider.endpoint_uri)
        print("Block Height:", block_height)
        print("Query Interval:", query_interval)
        print("-" * 68)
        print("Available Balance: R5 {:.4f}".format(balance))
        print("-" * 68)
        print("1. Send Transaction")
        print("2. Refresh Wallet")
        print("3. Transaction History")
        print("4. Expose Private Key")
        print("5. Reset Wallet (!!)")
        print("6. Exit")
        print("-" * 68)
        # Use timed_input so that if no option is entered in query_interval seconds, the page refreshes.
        choice = timed_input(">", query_interval)
        print("-" * 68)
        if choice == '1':
            send_tx(w3, wallet)
        elif choice == '2':
            continue  # Simply refreshes the menu (which now shows updated block height and balance)
        elif choice == '3':
            show_tx_history(w3, wallet)
        elif choice == '4':
            expose_private_key(wallet)
        elif choice == '5':
            reset_wallet()
        elif choice == '6':
            print("Exiting R5 CLI Wallet.")
            pause()
            sys.exit(0)
        elif choice == "":
            continue
        else:
            print("Invalid option. Please select a valid menu option.")
            time.sleep(1)

# -------------------------------
# Main Function
# -------------------------------
def main():
    settings = load_settings()
    rpc_address = settings.get("rpc_address", DEFAULT_RPC_ADDRESS)
    try:
        query_interval = int(settings.get("query_interval", DEFAULT_QUERY_INTERVAL))
    except ValueError:
        query_interval = DEFAULT_QUERY_INTERVAL
    w3 = Web3(Web3.HTTPProvider(rpc_address))
    if not w3.is_connected():
        print("Error: Unable to connect to the RPC address:", rpc_address)
        pause()
        sys.exit(1)
    print("Connected to RPC at", rpc_address)
    clear_screen()
    print_header()
    wallet = wallet_setup()
    run_main_menu(w3, wallet, query_interval)

if __name__ == '__main__':
    main()
