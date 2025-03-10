# R5 Core

R5 revisits the proof‑of‑work consensus mechanism to create a high‑performance, secure blockchain. It is designed to process over 1,000 transactions per second while establishing a fair economic dynamic for all participants. This repository contains the core implementation of the R5 Protocol, including the r5 node relayer and all accompanying SDK tools.

**This repository is constantly evolving; some information above may change over time. Please refer to the latest documentation for up‑to‑date details.**

## Hardware Requirements

Please note that these are the **minimum** hardware requirements for running each one of the node types specified. For more information, including **recommended** harware specs, please visit https://docs.r5.network/for-developers/hardware-requirements.

| Description | Archive Nodes  | Full Nodes      | Light Nodes    |
| ----------- | -------------- | --------------- | -------------- |
| CPU Cores   | 12             | 4               | 2              |
| RAM         | 10 GB          | 8 GB            | 2 GB           |
| Storage     | SSD 1 TB       | SSD 240 GB      | HDD 1GB        |
| Network     | Stable 125MB/s | Stable 100MB/s  | Stable 10MB/s  |

## Pre-Requisites

You will need to have `Python`, `Golang`, and a `C` compiler to build the core binary, and additional Python dependencies when building the full set of protocol tools.

If you prefer, the `install.py` script installs all required dependencies automatically. If you are running Windows, you might need to install a C compiler separately, even after running the install script.

You can run:

```bash
python3 install.py
```

Or, if on Windows:

```cmd
python install.py
```

## Building R5

You can build the binaries by running the `build.py` script. To compile only the core node binary, run the script with the `--coreonly` flag. If no flag is provided, the script will build the full R5 Protocol with all its tools.

Run on Linux/macOS:

```bash
python3 build.py
```

Or on Windows:

```cmd
python build.py
```

### Full Protocol Build

This build compiles all the tools included with the R5 Protocol, giving you a fully functioning node in your `/build` folder. The main `r5` binary is the custom node relayer, and inside the `/build/bin` folder you will find:

| File       | Description                                                                                                                                              |
|------------|----------------------------------------------------------------------------------------------------------------------------------------------------------|
| cliwallet  | A CLI wallet module that can generate wallet addresses, send transactions, and perform other wallet functions.                                           |
| node       | The core binary of the R5 node. It can be executed separately and accepts flags, though we recommend starting the node via the provided relayer.         |
| proxy      | A native SSL proxy for RPC operators. It accepts the --gencert flag to generate self-signed certificates and can serve requests via the SSL port.       |
| console  | R5's custom CLI console module, offering a user-friendly interface even for beginners.                                                                   |

For more information about each tool, please visit R5's documentation library at https://docs.r5.network.

#### The r5 Relayer

Inside the `/build` folder, you will find the main node binary, named `r5` (or `r5.exe` on Windows). This is the recommended entry point for your R5 node. The relayer simplifies node startup by consolidating the traditional `Geth` flag structure into an easier-to-use set of initialisation options.

| Flag                | Parameters                                  | Default   | Description                                                                                                                                                          |
|---------------------|---------------------------------------------|-----------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `-h` or `--help`        | n/a                                         | n/a       | Displays a list of supported flags and information. This flag must be used alone.                                                                                    |
| `--network`           | `mainnet`, `testnet`, `devnet`, `local`              | `mainnet`   | Defines the network used to initialise the node. Local networks use chain ID `13512` by default.                                                                         |
| `--rpc`               | n/a                                         | disabled  | Enables the node's RPC service (HTTP, WS, and GraphQL endpoints) with `web3`, `eth`, `r5`, and `net` namespaces. Recommended for use with the SSL proxy in production.  |
| --node              | archive, full, light                        | full      | Determines the type of node to run.                                                                                                                                |
| `--miner`             | `coinbase`, `threads`                           | disabled  | Enables mining and defines the coinbase address and number of CPU threads to use.                                                                                  |
| `--jsconsole`         | n/a                                         | n/a       | Opens the R5 JS Console. This flag must be used alone.                                                                                                             |
| `--proxy`             | `gencert` (optional)                          | disabled  | Starts the SSL Proxy service. Forwards HTTPS requests (default from port `443` to port `8545`). Use the gencert flag to generate self-signed certificates. Must be used alone. |
| `--r5console`         | n/a                                         | n/a       | Starts the built-in R5 Console. This flag must be used alone.                                                                                                      |
| `--bypass`            | n/a                                         | n/a       | Bypasses commands directly to the node binary for advanced configuration. Must be used alone.                                                                        |
| `--cliwallet`         | n/a                                         | n/a       | Starts the CLI Wallet. Must be used alone.                                                                                                                         |

**Usage Example:** Starting a mainnet RPC archive node with mining enabled:

```bash
./r5 --network mainnet --rpc --node archive --miner coinbase=0xABC... threads=1
```

## Configuration File "node.ini"

For faster and more consistent deployment, you can pre-configure a `node.ini` file and place it in the same folder as your r5 binary. This file acts as a preset for node initialisation settings. Any flags provided during startup will override the node.ini settings.

### Example node.ini File

```ini
[R5 Node Relayer]
network = mainnet
rpc = true
mode = default
miner = true
miner.coinbase = 0xABC...
miner.threads = 1
genesis = default
config = default
```

## Folder Structure

The `/build` folder should contain the following subdirectories:

| Subdirectory | Description                                                                                     |
|--------------|-------------------------------------------------------------------------------------------------|
| `/bin`         | Contains the main binaries and their dependencies, including configuration files and tools.   |
| `/config`      | Contains configuration files for R5 networks.                                                 |
| `/genesis`        | Contains genesis files for testnet, devnet, and local networks.                                 |

Additional directories, such as the main data storage folder, will be created when the node starts.

## Core Binary Build

If you prefer to build only the core protocol binary, run:

```bash
python3 build.py --coreonly
```

After building, the binary will be located in the /build folder. Note that this build does not utilise the r5 node relayer and will require the standard Geth flag structure. It does not offer ready-support for running testnet or devnet nodes, so we do not recommend this method unless you are very familiar with Geth.

## Running Your Node

After building, start your node using the r5 Relayer with the desired configuration. For example, to start a mainnet RPC archive node with mining enabled, run:

```bash
./r5 --network mainnet --rpc --node archive --miner coinbase=0xABC... threads=1
```

You may also configure initialisation parameters using the node.ini file for a more streamlined deployment.

## SDK Tools

The R5 SDK suite provides a collection of tools to enhance node deployment and management:

- **SSL Proxy:**  Provides native SSL support for RPC nodes and includes a built-in tool for generating self‑signed certificates.
- **R5 Console:** A user-friendly CLI console for node operators.
- **JS Console:** Grants direct access to the node’s JavaScript runtime and full control over all RPC API namespaces.
- **CLI Wallet:** A command-line wallet for executing transactions and managing multiple wallet addresses.
- **SCdev:** A tool for deploying and interacting with smart contracts on the R5 Network (currently under development).

For further details and tutorials, please visit [R5 Documentation](https://docs.r5.network).
