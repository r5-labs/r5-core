#!/usr/bin/env python3
"""
Production-Ready Ethash-R5 Library

This module implements Ethash-R5, which is adapted from our Go miner.
It provides:

  • Keccak-512 and Keccak-256 functions using PyCryptodome.
  • FNV-inspired mixing functions.
  • Cache generation (using the same constants as our Go miner).
  • Dataset item generation.
  • The core hashimoto() and hashimoto_light() functions.

Constants (such as EPOCH_LENGTH, MIX_BYTES, HASH_BYTES, etc.) and the
lookup arrays CACHE_SIZES and DATASET_SIZES are taken from the Go source.
Note: For production use, the CACHE_SIZES and DATASET_SIZES arrays should be
fully populated (here we include sample values for early epochs).
"""

import math
import struct
from Crypto.Hash import keccak

# ---------------- Global Constants ----------------
DATASET_INIT_BYTES   = 1 << 30    # 1GB at genesis
DATASET_GROWTH_BYTES = 1 << 23
CACHE_INIT_BYTES     = 1 << 24
CACHE_GROWTH_BYTES   = 1 << 17
EPOCH_LENGTH         = 30000
MIX_BYTES            = 128        # bytes
HASH_BYTES           = 64         # output of keccak512 in bytes
HASH_WORDS           = 16         # 64 bytes => 16 words (32-bit each)
DATASET_PARENTS      = 256
LOOP_ACCESSES        = 64
EXTRA_ROUNDS         = 4
MAX_EPOCH            = 2048

# For production, supply full arrays covering all epochs.
CACHE_SIZES = [
    16776896, 16907456, 17039296, 17170112, 17301056,
    17432512, 17563072, 17693888, 17824192, 17955904,
    # ... additional values as needed
]

DATASET_SIZES = [
    1073739904, 1082130304, 1090514816, 1098906752, 1107293056,
    1115684224, 1124070016, 1132461952, 1140849536, 1149232768,
    # ... additional values as needed
]

# ---------------- Keccak Hash Helpers ----------------
def keccak_512(data: bytes) -> bytes:
    return keccak.new(digest_bits=512, data=data).digest()

def keccak_256(data: bytes) -> bytes:
    return keccak.new(digest_bits=256, data=data).digest()

# ---------------- Utility Functions ----------------
def int_to_le(n: int) -> bytes:
    return n.to_bytes(4, byteorder="little")

def le_to_int(b: bytes) -> int:
    return int.from_bytes(b, byteorder="little")

# ---------------- FNV Mixing ----------------
def fnv(a: int, b: int) -> int:
    return ((a * 0x01000193) & 0xffffffff) ^ b

def fnv_hash(mix: list, data: list):
    for i in range(len(mix)):
        mix[i] = fnv(mix[i], data[i])

# ---------------- Cache Generation ----------------
def calc_cache_size(epoch: int) -> int:
    size = CACHE_INIT_BYTES + CACHE_GROWTH_BYTES * epoch - HASH_BYTES
    return size

def generate_cache(seed: bytes, block_number: int) -> list:
    """
    Generate the verification cache for a given block number using the provided seed.
    """
    epoch = block_number // EPOCH_LENGTH
    if epoch < len(CACHE_SIZES):
        cache_size = CACHE_SIZES[epoch]
    else:
        cache_size = calc_cache_size(epoch)
    cache_bytes = bytearray(cache_size)
    # First 64 bytes = keccak512(seed)
    cache_bytes[0:HASH_BYTES] = keccak_512(seed)
    for offset in range(HASH_BYTES, cache_size, HASH_BYTES):
        prev = cache_bytes[offset - HASH_BYTES: offset]
        cache_bytes[offset: offset+HASH_BYTES] = keccak_512(bytes(prev))
    rows = cache_size // HASH_BYTES
    for r in range(3):  # CACHE_ROUNDS = 3
        for j in range(rows):
            srcOff = ((j - 1 + rows) % rows) * HASH_BYTES
            dstOff = j * HASH_BYTES
            val = int.from_bytes(cache_bytes[dstOff:dstOff+4], "little")
            xorOff = (val % rows) * HASH_BYTES
            block = bytes(a ^ b for a, b in zip(cache_bytes[srcOff:srcOff+HASH_BYTES],
                                                  cache_bytes[xorOff:xorOff+HASH_BYTES]))
            cache_bytes[dstOff:dstOff+HASH_BYTES] = keccak_512(block)
    # Convert the cache bytes into a list of little-endian 32-bit unsigned ints.
    cache = []
    for i in range(0, len(cache_bytes), 4):
        cache.append(int.from_bytes(cache_bytes[i:i+4], "little"))
    return cache

# ---------------- Dataset Item Generation ----------------
def generate_dataset_item(cache: list, index: int) -> bytes:
    """
    Compute one dataset item (64 bytes) from the cache.
    """
    rows = len(cache) // HASH_WORDS
    mix = bytearray()
    base = (index % rows) * HASH_WORDS
    first = cache[base] ^ index
    mix.extend(int_to_le(first))
    for i in range(1, HASH_WORDS):
        mix.extend(int_to_le(cache[base + i]))
    mix = keccak_512(bytes(mix))
    # Convert mix to a list of 16 uint32 integers.
    mix_int = [le_to_int(mix[i*4:(i+1)*4]) for i in range(HASH_WORDS)]
    for i in range(DATASET_PARENTS):
        parent = fnv(index ^ i, mix_int[i % 16]) % rows
        parent_offset = parent * HASH_WORDS
        parent_words = cache[parent_offset:parent_offset+HASH_WORDS]
        mix_int = [fnv(mix_int[k], parent_words[k]) for k in range(HASH_WORDS)]
    mixed = b"".join(int_to_le(w) for w in mix_int)
    return keccak_512(mixed)

# ---------------- Core Hashimoto Algorithm ----------------
def hashimoto(header: bytes, nonce: int, size: int, lookup) -> (bytes, bytes):
    """
    Compute the Ethash-R5 hashimoto.
      header: 32-byte header (powHash)
      nonce: 64-bit integer nonce
      size: dataset size in bytes
      lookup: function(index: int) -> list of 16 uint32 integers (dataset item)
    Returns: (final_digest, final_hash) as bytes
    """
    rows = size // MIX_BYTES
    # Construct seed: header || nonce (8-byte LE)
    seed = header + nonce.to_bytes(8, "little")
    seed = keccak_512(seed)
    seed_head = int.from_bytes(seed[0:4], "little")
    mix = []
    for i in range(MIX_BYTES // 4):
        pos = (i % 16) * 4
        mix.append(int.from_bytes(seed[pos:pos+4], "little"))
    temp = mix.copy()
    for i in range(LOOP_ACCESSES):
        parent = fnv(i ^ seed_head, mix[i % len(mix)]) % rows
        num_chunks = MIX_BYTES // HASH_BYTES  # should be 2 chunks for 128 bytes
        for j in range(num_chunks):
            item = lookup(2 * parent + j)
            for k in range(HASH_WORDS):
                temp[j*HASH_WORDS + k] = item[k]
        fnv_hash(mix, temp)
    # Collapse mix into 4 uint32 values
    collapsed = []
    for i in range(0, len(mix), 4):
        collapsed.append(fnv(fnv(fnv(mix[i], mix[i+1]), mix[i+2]), mix[i+3]))
    digest = b"".join(int_to_le(x) for x in collapsed)
    extra = bytearray(digest)
    for _ in range(EXTRA_ROUNDS):
        for i in range(0, len(extra), 4):
            word = int.from_bytes(extra[i:i+4], "little")
            seed_word = int.from_bytes(seed[((i//4)%10)*4:((i//4)%10)*4+4], "little")
            word = fnv(word, seed_word)
            extra[i:i+4] = int_to_le(word)
        extra = bytearray(keccak_256(bytes(extra)))
    final_digest = bytes(extra)
    final_hash = keccak_256(seed + final_digest)
    return final_digest, final_hash

def hashimoto_light(header: bytes, nonce: int, cache: list) -> (bytes, bytes):
    """
    Light version that uses only the cache.
    Dataset size is assumed to be len(cache)*4.
    """
    size = len(cache) * 4
    def lookup(index: int) -> list:
        item = generate_dataset_item(cache, index)
        return [le_to_int(item[i*4:(i+1)*4]) for i in range(HASH_WORDS)]
    return hashimoto(header, nonce, size, lookup)

# ---------------- Seed Hash Calculation ----------------
def seed_hash(block_number: int) -> bytes:
    """
    Compute the seed hash for the given block number by iteratively keccak256-hashing.
    """
    seed = bytes(32)
    if block_number < EPOCH_LENGTH:
        return seed
    rounds = block_number // EPOCH_LENGTH
    for _ in range(rounds):
        seed = keccak_256(seed)
    return seed

if __name__ == "__main__":
    # For testing the module.
    print("Ethash-R5 module loaded. Use this module from r5miner.py.")
