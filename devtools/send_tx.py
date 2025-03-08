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

"""
A simple Python script to send a hardcoded raw transaction via a local node.

Hardcoded values:
  - Sender address:    0x288be778b666Ed006357ce12f455fbB3C7D0Ec94
  - Recipient address: 0x9d711bfc6e9B46B82485C622b493dF16ab961066
  - Private key:       0xYOUR_PRIVATE_KEY_HERE (replace this placeholder with your actual key in hex)
  - Amount:            1 R5 test transaction (1 ETH equivalent, i.e. 1e18 wei)
  - Gas Price:         10 gwei
  - Gas Limit:         21000

The script prompts for confirmation before sending each transaction.
"""

import sys
from web3 import Web3

# Connect to the local node (adjust the URL if needed)
w3 = Web3(Web3.HTTPProvider("http://127.0.0.1:8545"))
if not w3.is_connected():
    print("Error: Unable to connect to the local node!")
    sys.exit(1)

# Hardcoded parameters
SENDER = "0x288be778b666Ed006357ce12f455fbB3C7D0Ec94"
DESTINATION = "0x9d711bfc6e9B46B82485C622b493dF16ab961066"
# THIS PRIVATE KEY IS NOT SENSITIVE AND MEANT TO BE HERE FOR TESTING!
PRIVATE_KEY = "0x9d845309f5edfbf973fa59701a4998942afac8fea78034d45d91004854ed1456"  # Replace with your actual private key in hex format.
AMOUNT = w3.to_wei(1, "ether")           # 1 R5 test transaction (1 ETH equivalent)
GAS_PRICE = w3.to_wei(10, "gwei")
GAS_LIMIT = 21000

def send_transaction():
    try:
        nonce = w3.eth.get_transaction_count(SENDER)
    except Exception as e:
        print("Error fetching nonce:", e)
        return

    tx = {
        "nonce": nonce,
        "to": DESTINATION,
        "value": AMOUNT,
        "gas": GAS_LIMIT,
        "gasPrice": GAS_PRICE,
        # Optionally add "chainId": <your_chain_id>
    }

    # Validate the private key format before signing.
    key_to_use = PRIVATE_KEY.strip()
    if not key_to_use.startswith("0x"):
        key_to_use = "0x" + key_to_use
    try:
        # This will raise an exception if non-hexadecimal characters are found.
        bytes.fromhex(key_to_use[2:])
    except Exception:
        print("Invalid private key. Please replace the placeholder with a valid hex private key (with '0x' prefix).")
        return

    try:
        signed_tx = w3.eth.account.sign_transaction(tx, key_to_use)
    except Exception as e:
        print("Error signing transaction:", e)
        return

    try:
        # Use the correct attribute 'raw_transaction'
        tx_hash = w3.eth.send_raw_transaction(signed_tx.raw_transaction)
        print("Transaction sent! Tx hash:", w3.to_hex(tx_hash))
        receipt = w3.eth.wait_for_transaction_receipt(tx_hash, timeout=120)
        print("Transaction receipt:", receipt)
    except Exception as e:
        print("Error sending transaction:", e)

def main():
    while True:
        prompt = (
            f"Confirm the sending of a 1 R5 test transaction from {SENDER} to {DESTINATION}? (y/n): "
        )
        confirm = input(prompt).strip().lower()
        if confirm == "y":
            send_transaction()
        else:
            print("Transaction not sent.")
        cont = input("Do you want to send another transaction? (y/n): ").strip().lower()
        if cont != "y":
            break

if __name__ == "__main__":
    main()
