#!/usr/bin/env python3
from decimal import Decimal, getcontext

# Use high precision for fractional rewards
getcontext().prec = 28

# Constants
BLOCK_TIME_SEC = Decimal('7')               # average block time
SECONDS_PER_YEAR = Decimal('365') * 24 * 60 * 60
BLOCKS_PER_YEAR = int(SECONDS_PER_YEAR / BLOCK_TIME_SEC)

PREMINED = Decimal('2000000')               # genesis allocation
SUPPLY_CAP = Decimal('66337700')            # total max supply
TOTAL_BLOCK_REWARDS = SUPPLY_CAP - PREMINED

# Emission epochs: (start_block, end_block_or_None, reward_per_block in R5)
EPOCHS = [
    (1,          4_000_000,   Decimal('2')),
    (4_000_001,  8_000_000,   Decimal('1')),
    (8_000_001, 16_000_000,   Decimal('0.5')),
    (16_000_001, 32_000_000,  Decimal('0.25')),
    (32_000_001, 64_000_000,  Decimal('0.125')),
    (64_000_001,128_000_000,  Decimal('0.0625')),
    (128_000_001, None,       Decimal('0.03125')),  # runs until cap
]

def main():
    year = 1
    start_block = 1
    cumulative_rewards = Decimal('0')
    lines = []

    while cumulative_rewards < TOTAL_BLOCK_REWARDS:
        end_block = start_block + BLOCKS_PER_YEAR - 1
        year_emitted = Decimal('0')
        cursor = start_block

        for epoch_start, epoch_end, reward in EPOCHS:
            if cursor > end_block:
                break

            # determine the last block of this epoch within the year
            last = end_block if epoch_end is None else min(end_block, epoch_end)
            first = max(cursor, epoch_start)
            if last < first:
                continue

            blocks = last - first + 1

            # check if this would exceed the cap
            remaining = TOTAL_BLOCK_REWARDS - cumulative_rewards
            max_blocks = int(remaining / reward)
            if blocks > max_blocks:
                blocks = max_blocks

            amount = reward * blocks
            year_emitted += amount
            cumulative_rewards += amount
            cursor += blocks

            if cumulative_rewards >= TOTAL_BLOCK_REWARDS:
                break

        total_supply = PREMINED + cumulative_rewards
        lines.append(
            f"Year {year}: Emitted {year_emitted} R5, "
            f"Cumulative Block Rewards {cumulative_rewards} R5, "
            f"Total Supply {total_supply} R5"
        )

        year += 1
        start_block = end_block + 1

    # write results to file
    with open("reward_calculation.txt", "w") as f:
        for line in lines:
            f.write(line + "\n")

if __name__ == "__main__":
    main()
