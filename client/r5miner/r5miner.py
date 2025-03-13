#!/usr/bin/env python3
"""
Production-Ready Ethash-R5 CPU Miner

Usage:
  python r5miner.py -p <rpc_url> -a <reward_address> -cpu <cores> -w <worker_name>

This miner:
  • Retrieves work via eth_getWork.
  • Queries the current block number via eth_blockNumber.
  • Generates the cache (using the node-provided seed hash) and then
    uses our Ethash-R5 implementation to search for a valid nonce.
  • When a valid nonce is found (i.e. final_hash < target), it submits
    the solution via eth_submitWork.
  • It then repeats with new work.
  
Dependencies:
  pip install requests pycryptodome
"""

import argparse
import logging
import os
import sys
import time
from multiprocessing import Process

import requests

from ethash_r5 import (
    hashimoto_light,
    generate_cache,
    seed_hash,
    EPOCH_LENGTH,
    HASH_WORDS,
    MIX_BYTES,
)

# --- Logging Setup ---
logging.basicConfig(
    level=logging.INFO,
    format="[%(asctime)s] [%(processName)s] %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
)
logger = logging.getLogger(__name__)

# --- JSON-RPC Utilities ---
def json_rpc_request(url: str, method: str, params):
    payload = {"jsonrpc": "2.0", "method": method, "params": params, "id": 1}
    try:
        response = requests.post(url, json=payload, timeout=10)
        response.raise_for_status()
        data = response.json()
        if "error" in data:
            logger.error(f"[RPC ERROR] {method}: {data['error']}")
            return None
        return data.get("result")
    except Exception as e:
        logger.error(f"[RPC ERROR] {method}: {e}")
        return None

def get_work(rpc_url: str):
    result = json_rpc_request(rpc_url, "eth_getWork", [])
    if not result or len(result) < 3:
        raise Exception("Invalid work received")
    # Work is returned as [powHash, seedHash, target]
    return result[0], result[1], int(result[2], 16)

def get_block_number(rpc_url: str) -> int:
    result = json_rpc_request(rpc_url, "eth_blockNumber", [])
    if not result:
        raise Exception("Failed to get block number")
    return int(result, 16)

def submit_work(rpc_url: str, nonce: int, mix_digest: bytes, pow_hash: str) -> bool:
    nonce_hex = f"0x{nonce:016x}"
    mix_hex = "0x" + mix_digest.hex()
    params = [nonce_hex, mix_hex, pow_hash]
    result = json_rpc_request(rpc_url, "eth_submitWork", params)
    if isinstance(result, bool):
        return result
    elif isinstance(result, str) and result.lower() == "true":
        return True
    return False

# --- Miner Worker Process ---
def miner_worker(worker_id: int, rpc_url: str, reward_addr: str, worker_name: str):
    logger.info(f"[Worker {worker_id} - {worker_name}] Starting mining loop.")
    while True:
        # Retrieve work
        try:
            pow_hash_str, seed_hash_str, target = get_work(rpc_url)
            logger.info(f"[Worker {worker_id}] Work received: header={pow_hash_str}, target={hex(target)}")
        except Exception as e:
            logger.error(f"[Worker {worker_id}] Error retrieving work: {e}")
            time.sleep(5)
            continue

        try:
            header_bytes = bytes.fromhex(pow_hash_str[2:])  # remove "0x"
            seed_hash_bytes = bytes.fromhex(seed_hash_str[2:])
        except Exception as e:
            logger.error(f"[Worker {worker_id}] Error converting work to bytes: {e}")
            time.sleep(5)
            continue

        # Get current block number to determine cache size
        try:
            block_number = get_block_number(rpc_url)
        except Exception as e:
            logger.error(f"[Worker {worker_id}] Error retrieving block number: {e}")
            time.sleep(5)
            continue

        try:
            cache = generate_cache(seed_hash_bytes, block_number)
        except Exception as e:
            logger.error(f"[Worker {worker_id}] Error generating cache: {e}")
            time.sleep(5)
            continue

        nonce = worker_id  # Different starting nonce per worker
        max_nonce = 2**64 - 1
        start_time = time.time()
        iterations = 0
        solution_found = False

        # Main mining loop for this work
        while nonce < max_nonce:
            iterations += 1
            try:
                mix_digest, final_hash = hashimoto_light(header_bytes, nonce, cache)
            except Exception as e:
                logger.error(f"[Worker {worker_id}] Error in hashimoto_light: {e}")
                break

            if int.from_bytes(final_hash, byteorder="big") < target:
                elapsed = time.time() - start_time
                logger.info(f"[Worker {worker_id}] SUCCESS: Nonce found: {nonce}")
                logger.info(f"[Worker {worker_id}] Final hash: {final_hash.hex()}")
                logger.info(f"[Worker {worker_id}] Reward address: {reward_addr}")
                logger.info(f"[Worker {worker_id}] Iterations: {iterations} in {elapsed:.2f} sec")
                if submit_work(rpc_url, nonce, mix_digest, pow_hash_str):
                    logger.info(f"[Worker {worker_id}] Solution submitted successfully.")
                else:
                    logger.error(f"[Worker {worker_id}] Failed to submit solution.")
                solution_found = True
                break

            # Increment nonce by number of CPU cores to avoid overlap among workers
            nonce += os.cpu_count()

        if solution_found:
            logger.info(f"[Worker {worker_id}] Solution processed; requesting new work...")
        else:
            logger.info(f"[Worker {worker_id}] No valid solution found; re-requesting work...")
        time.sleep(2)

# --- Main Miner Process ---
def main():
    parser = argparse.ArgumentParser(description="Production-Ready Ethash-R5 Miner (CPU)")
    parser.add_argument("-p", type=str, required=True, help="Node RPC address (e.g. http://127.0.0.1:8545)")
    parser.add_argument("-a", type=str, required=True, help="Reward address (your wallet address)")
    parser.add_argument("-cpu", type=int, default=os.cpu_count(), help="Number of CPU cores to use for mining")
    parser.add_argument("-w", type=str, default="Worker", help="Worker name identifier")
    args = parser.parse_args()

    try:
        pow_hash_str, seed_hash_str, target = get_work(args.p)
        logger.info(f"Initial work: header={pow_hash_str}, target={hex(target)}")
    except Exception as e:
        logger.error(f"Error obtaining initial work: {e}")
        sys.exit(1)

    workers = []
    for i in range(args.cpu):
        p = Process(target=miner_worker, args=(i, args.p, args.a, args.w))
        p.start()
        workers.append(p)
    for p in workers:
        p.join()

if __name__ == "__main__":
    main()
