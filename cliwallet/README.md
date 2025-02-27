# R5 CLI Wallet

The R5 CLI Wallet is a simple and easy-to-use wallet for the R5 Network. It is capable of connecting to your local node or a remote RPC server and has basic functionality that allows you to create and manage your digital wallets.

Currently, with the R5 CLI Wallet you can:

- Create and mange one or more R5 wallets with encryption
- Send and receive native transactions
- View your transaction history (for the past 1080 blocks)

## Usage

To start the wallet, simply double-click the executable file or call it via terminal using `.\r5wallet` on Windows or `./r5wallet` on Linux and macOS.

**Before starting the wallet**, you can import any existing R5 CLI Wallet by placing its r5.key file inside the wallet's directory.

**If you're not running a local node** you will need to create a settings file to specify your RPC URL (more instructions below).

When starting the wallet for the first time you will be asked if you want to **import an existing wallet using its private key**. If you want to create a fresh new wallet, just respond `n` to the above.

### Main Page

On the main page you will see key information about your wallet, the RPC, and the network, as well as the main menu with all the available functions of the wallet:

* **Send Transaction**: Used to send funds.
* **Refresh Wallet**: Refreshes your balance and the network block height.
* **Transaction History**: Displays the history of transactions for the past 1080 blocks.
* **Expose Private Key**: Exposes your private key. (CAUTION!)
* **Reset Wallet**: Used to wipe and reset the wallet. You can backup your existing wallet by copying the `r5.key` file to a safe folder.

### Sending Transactions

Sending transactions is very simple, just enter the option `1` on the main menu, and follow the instructions on the screen.

1) On the `To:` field, paste or write the address you want to send funds to.
2) On the `For:` field, specify the amount of R5 coins you want to send (**IMPORTANT!** use `.` for decimals, for example, `10.5`),.
3) For transaction fees, you can either leave the fields blank to let the wallet calculate the optimal fees automatically, or manually specify the maximum fee and the gas price for the transaction.

After that, all you need to do is to confirm the transaction to send it to the blockchain.

## Storage and Folder Structure

There are 3 main files you will find inside the R5 CLI Wallet directory:

1) `r5wallet` - The actual wallet application.
2) `r5.key` - Your encrypted walelt file.
3) `settings.r5w` - Your settings file.

### r5wallet

The wallet's binary. You can double-click it to start the application, or call it via the terminal (eg. `.\r5wallet` on Windows or `./r5wallet` on Linux and macOS).

### r5.key

This file can be understood as being your actual wallet. It contain encrypted information that can only be accessed with your encryption password. **You can move this file between different folders and devices to access your wallet from multiple R5 CLI Wallet instances.**

## settings.r5w

The wallet's settings are stored here. If you do not create your own settings file before starting the wallet, it will use the default parameters. The available variables are:

- `rpc_address`: RPC URL (default: http://localhost:8545)
- `query_interval` = Refresh interval for the wallet (default: 60)
- `tx_timeout` = Timeout when sending transactions (default: 1200)

Formatting example:

```bash
# r5w-start
rpc_address = http://localhost:8545
query_interval = 60
tx_timeout = 1200
# r5w-end
```

* Note that you need to start the file with `# r5w-start` and end the file with `# r5w-end`
