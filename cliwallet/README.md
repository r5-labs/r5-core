# R5 CLI Wallet

The R5 CLI Wallet is a command-line wallet for the R5 Network, designed to be simple and efficient. It allows users to manage their funds, connect to local or remote RPC nodes, and perform essential wallet functions.

## Features

- Create and manage multiple encrypted R5 wallets
- Send and receive native transactions
- View transaction history (up to the last 1080 blocks)

## Getting Started

### Running the Wallet

To start the wallet:

- **Windows:** Double-click the executable (`r5wallet.exe`) or run it from the terminal:
  ```
  .\r5wallet
  ```
- **Linux/macOS:** Open a terminal and run:
  ```
  ./r5wallet
  ```

### Importing an Existing Wallet

If you have an existing R5 CLI Wallet, place the `r5.key` file in the wallet’s directory before launching the application.

### First-Time Setup

Upon first launch, the wallet will prompt you to:

1. **Import an existing wallet using a private key**
2. **Create a new wallet with encryption**

If you choose to create a new wallet, you will be required to set an encryption password.

### Connecting to a Remote RPC

By default, the wallet connects to `http://localhost:8545`. If you are not running a local node, you must edit the settings file (`settings.r5w`) to specify a different RPC URL. See the [Settings](#settingsr5w) section for more details.

## Using the Wallet

### Main Menu

The main menu provides access to all key functions:

- **1. Send Transaction** - Transfer R5 to another address
- **2. Refresh Wallet** - Update balance and network block height
- **3. Transaction History** - View transactions from the past 1080 blocks
- **4. Expose Private Key** - Display private key (use with caution)
- **5. Reset Wallet** - Wipe and reset the wallet (backup `r5.key` first)
- **6. Exit** - Close the wallet

### Sending Transactions

To send R5, select **"Send Transaction"** from the main menu and follow these steps:

1. Enter the recipient’s address.
2. Specify the amount of R5 to send. Use `.` for decimals (e.g., `10.5`).
3. Choose transaction fees:
   - Leave blank for automatic calculation.
   - Manually enter max gas fee and gas price if needed.
4. Confirm the transaction to send it to the blockchain.

## Storage and File Structure

The R5 CLI Wallet consists of three main files:

| File         | Description |
|-------------|-------------|
| `r5wallet`  | The wallet executable binary. |
| `r5.key`  | The encrypted wallet file containing private keys. |
| `settings.r5w`  | Configuration file for customizing wallet settings. |

## settings.r5w

This file contains wallet settings. If it does not exist, the wallet will create one with default values.

### Available Settings

| Setting | Description | Default Value |
|---------|-------------|---------------|
| `rpc_address` | The RPC URL to connect to | `http://localhost:8545` |
| `query_interval` | How often the wallet refreshes (in seconds) | `60` |
| `tx_timeout` | Timeout for transaction confirmations (in seconds) | `1200` |

### Example settings file (`settings.r5w`):

```bash
# r5w-start
rpc_address = http://localhost:8545
query_interval = 60
tx_timeout = 1200
# r5w-end
```

The file must begin with `# r5w-start` and end with `# r5w-end`.

## Security Notice

- Never share your private key or expose your `r5.key` file.
- Always back up your `r5.key` file in a secure location.
- If you need to recover your wallet, copy `r5.key` to the wallet directory.

## Troubleshooting and Support

For troubleshooting and support, visit the R5 Network community or check the documentation.
