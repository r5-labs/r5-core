#! /usr/bin/env python3
# Copyright 2025 R5 Labs
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
A simple Python script to deposit 1 R5 into the ZKNet contract by calling
the deposit function (r5_balanceDeposit). The script uses a user-provided private key,
builds the contract call transaction, signs it, and then sends the raw transaction.
"""

import sys
import json
from web3 import Web3

# Connect to your node (adjust the URL if needed)
w3 = Web3(Web3.HTTPProvider("https://rpc-devnet.r5.network"))
if not w3.is_connected():
    print("Error: Unable to connect to the node!")
    sys.exit(1)

# Hardcoded contract address (devnet contract)
contract_address = "0x1A52C4914F8A0c254C69699631a1C92De4cCf01A"

# Path to the compiled contract ABI
abi_path = "zknet.json"
with open(abi_path) as f:
    abi = json.load(f)

def deposit_r5():
    try:
        print("Checking contract...")
        contract = w3.eth.contract(address=contract_address, abi=abi)
        print(f"Contract address: {contract_address}")
        
        # Get the user's private key
        while True:
            private_key_input = input("\nEnter your private key (0x...): ").strip()
            if not private_key_input.startswith("0x"):
                print("Invalid private key format. It must start with '0x'.")
                continue
            try:
                bytes.fromhex(private_key_input[2:])
                break
            except ValueError:
                print("Invalid private key: could not decode as hex.")
                continue
                
        # Retrieve the account from the private key
        account = w3.eth.account.from_key(private_key_input)
        sender_address = account.address
        print(f"Sender address: {sender_address}")
        
        # Amount to deposit (1 R5 = 1e18 wei)
        amount = 1 * 10**18
        
        # Confirm the transaction
        confirm = input(
            f"\nDo you want to deposit 1 R5 to the ZKNet contract ({contract_address})? (yes/no): "
        ).strip().lower()
        if confirm != "yes":
            print("Deposit canceled.")
            return

        # Get the current nonce and chain ID
        nonce = w3.eth.get_transaction_count(sender_address)
        chain_id = w3.eth.chain_id
        gas_price = w3.to_wei(10, "gwei")
        
        # Build the transaction for the contract function call
        # Note: r5_balanceDeposit is payable so we must include the ether value.
        txn = contract.functions.r5_balanceDeposit(amount).build_transaction({
            "from": sender_address,
            "value": amount,
            "nonce": nonce,
            "chainId": chain_id,
            "gasPrice": gas_price,
        })
        
        # Optionally, estimate gas and update txn. If estimation fails, a default gas limit is used.
        try:
            estimated_gas = contract.functions.r5_balanceDeposit(amount).estimate_gas({
                "from": sender_address,
                "value": amount
            })
            txn["gas"] = estimated_gas
        except Exception as err:
            txn["gas"] = 21000
            print(f"Warning: Gas estimation failed, using default gas limit. ({err})")
        
        # Sign the transaction using the provided private key.
        signed_txn = w3.eth.account.sign_transaction(txn, private_key_input)
        
        # Use the raw_transaction attribute (note the underscore and lowercase)
        raw_tx = signed_txn.raw_transaction
        
        # Send the signed raw transaction
        tx_hash = w3.eth.send_raw_transaction(raw_tx)
        
        print("\nTransaction sent!")
        print("Transaction hash:", w3.to_hex(tx_hash))
        
        # Optionally wait for receipt
        receipt = w3.eth.wait_for_transaction_receipt(tx_hash, timeout=120)
        print("Transaction receipt:", receipt)
        return True

    except KeyboardInterrupt:
        print("\nUser interrupted. Deposit aborted.")
        return False
    except Exception as e:
        print(f"\nError depositing tokens: {e}")
        return False

if __name__ == "__main__":
    deposit_r5()
    