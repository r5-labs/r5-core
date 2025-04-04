#!/usr/bin/env python3
"""
Production-Ready Ethash-R5 CPU Miner

This miner:
  • Retrieves work via eth_getWork.
  • Queries the current block number via eth_blockNumber.
  • Generates the cache (using the node-provided seed hash) and then
    uses our Ethash-R5 implementation to search for a valid nonce.
  • When a valid nonce is found (i.e. final_hash < target), it checks if the block
    height is still current before submitting the solution via eth_submitWork.
  • If the block height has advanced, it logs a message and drops the work.
  • A centralized logging mechanism consolidates messages from all worker processes.
  
Dependencies:
  pip install requests pycryptodome
"""

import logging
import logging.handlers
import os
import sys
import time
import struct
import configparser
from multiprocessing import Process, Queue, freeze_support

import requests

# Define a global logger for use throughout the module.
logger = logging.getLogger(__name__)

# Import functions and constants from your Ethash-R5 implementation.
from ethash_r5 import (
    hashimoto_light,
    generate_cache,
    seed_hash,
    cache_size,
    dataset_size,
    EPOCH_LENGTH,
)

# --- Configuration Handling ---
def load_config():
    CONFIG_FILE = "miner.ini"
    default_config = {
        "pool_url": "http://localhost:8545",
        "cpu_threads": str(max(1, os.cpu_count() - 1)),
        "worker_name": "r5miner_worker"
    }
    config = configparser.ConfigParser()
    if not os.path.exists(CONFIG_FILE):
        config["Miner"] = default_config
        with open(CONFIG_FILE, "w") as configfile:
            config.write(configfile)
        print("Configuration file not detected. Created new miner.ini file, please configure it and restart the miner.")
        sys.exit(1)
    config.read(CONFIG_FILE)
    return config["Miner"]

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
    return result[0], result[1], int(result[2], 16)

def get_block_number(rpc_url: str) -> int:
    result = json_rpc_request(rpc_url, "eth_blockNumber", [])
    if not result:
        raise Exception("Failed to get block number")
    return int(result, 16)

def submit_work(rpc_url: str, nonce: int, mix_digest: bytes, pow_hash: str) -> bool:
    nonce_hex = f"0x{nonce:016x}"
    mix_hex = "0x" + mix_digest.hex()
    params = [nonce_hex, pow_hash, mix_hex]
    result = json_rpc_request(rpc_url, "eth_submitWork", params)
    return bool(result)

# --- Cache Generation Helper ---
def get_cache(seed: bytes, block_number: int) -> list:
    csize = cache_size(block_number)
    cache_buffer = bytearray(csize)
    epoch = block_number // EPOCH_LENGTH
    generate_cache(cache_buffer, epoch, seed)
    return [struct.unpack_from('<I', cache_buffer, i)[0] for i in range(0, len(cache_buffer), 4)]

# --- Worker Logging Setup ---
def setup_worker_logger(log_queue):
    root = logging.getLogger()
    for handler in root.handlers[:]:
        root.removeHandler(handler)
    queue_handler = logging.handlers.QueueHandler(log_queue)
    root.addHandler(queue_handler)
    root.setLevel(logging.INFO)

# --- Miner Worker Process ---
def miner_worker(worker_id: int, rpc_url: str, worker_name: str, total_workers: int, log_queue):
    setup_worker_logger(log_queue)
    logger = logging.getLogger(f"Worker-{worker_id}")
    logger.info(f"[Worker {worker_id} - {worker_name}] Starting mining loop.")

    # Cache optimization variables
    last_epoch = None
    cached_cache = None

    while True:
        try:
            pow_hash_str, seed_hash_str, target = get_work(rpc_url)
            header_bytes = bytes.fromhex(pow_hash_str[2:])
            seed_hash_bytes = bytes.fromhex(seed_hash_str[2:])
            current_block = get_block_number(rpc_url)
            work_epoch = current_block // EPOCH_LENGTH

            # Generate cache only if a new epoch has been reached.
            if work_epoch != last_epoch:
                cached_cache = get_cache(seed_hash_bytes, current_block)
                last_epoch = work_epoch
                logger.info(f"Cache generated for epoch {work_epoch}")
            else:
                logger.debug(f"Reusing cache for epoch {work_epoch}")

            cache = cached_cache
            ds = dataset_size(current_block)
            
            nonce = worker_id
            max_nonce = 2**64 - 1
            start_time = time.time()
            last_check = time.time()
            solution_found = False

            while nonce < max_nonce:
                # Check every 5 seconds if the block has advanced
                if time.time() - last_check >= 5:
                    current_height = get_block_number(rpc_url)
                    if current_height > current_block:
                        logger.info("Block advanced, dropping current work and getting new work...")
                        break
                    last_check = time.time()

                mix_digest, final_hash = hashimoto_light(ds, cache, header_bytes, nonce)
                if int.from_bytes(final_hash, byteorder="big") < target:
                    elapsed = time.time() - start_time
                    logger.info(f"SUCCESS: Nonce found: {nonce} in {elapsed:.2f}s")
                    # Check block height again before submitting the solution
                    current_height = get_block_number(rpc_url)
                    if current_height > current_block:
                        logger.info("Block advanced before submission, discarding found solution.")
                        break
                    if submit_work(rpc_url, nonce, mix_digest, pow_hash_str):
                        logger.info("Solution submitted successfully.")
                    else:
                        logger.error("Failed to submit solution.")
                    solution_found = True
                    break
                nonce += total_workers

            if not solution_found:
                logger.info("No valid solution found; re-requesting work...")
        except Exception as e:
            logger.error(f"Error in mining loop: {e}")
            time.sleep(5)
        time.sleep(2)

# --- Main Miner Process ---
def main():
    log_queue = Queue()
    handler = logging.StreamHandler(sys.stdout)
    formatter = logging.Formatter("[%(asctime)s] %(name)s %(message)s", datefmt="%Y-%m-%d %H:%M:%S")
    handler.setFormatter(formatter)
    listener = logging.handlers.QueueListener(log_queue, handler)
    listener.start()

    config = load_config()
    rpc_url = config.get("pool_url", "http://localhost:8545")
    cpu_threads = int(config.get("cpu_threads", str(max(1, os.cpu_count() - 1))))
    worker_name = config.get("worker_name", "r5miner_worker")

    main_logger = logging.getLogger("Main")
    try:
        pow_hash_str, seed_hash_str, target = get_work(rpc_url)
        main_logger.info(f"Initial work: header={pow_hash_str}, target={hex(target)}")
    except Exception as e:
        main_logger.error(f"Error obtaining initial work: {e}")
        sys.exit(1)

    workers = []
    for i in range(cpu_threads):
        p = Process(target=miner_worker, args=(i, rpc_url, worker_name, cpu_threads, log_queue))
        p.start()
        workers.append(p)
    for p in workers:
        p.join()
    listener.stop()

if __name__ == "__main__":
    freeze_support()  # Needed for Windows when using multiprocessing with PyInstaller
    main()
