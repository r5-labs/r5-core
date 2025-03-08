# R5 Core

Core implementation of the R5 Protocol.

## Pre-Requisites

You will need to have `Golang` and a `C` compiler to build the core binary, and additional `Python` dependencies will be required when building the protocol full set of tools.

If you prefer, the `install.py` script installs all required dependencies automatically. If you are running Windows, you might have to install the `C` compiler separately, even after running the install script.

You can run:

```bash
python3 install.py
```

Or, if on Windows:

```cmd
python install.py
```

## Building R5

You can build the binaries by running the `build.py` script. If you want to compile only the core node binary, you can run the script with the `--coreonly` flag. If no flag is received, the script will build the full R5 Protocol with all its tools.

Run:

```bash
python3 build.py
```

Or, if on Windows:

```cmd
python build.py
```

### Full Protocol Build

This will build the binaries of all the tools included with the R5 Protocol, and you will have a fully functioning and complete node built inside your `/build` folder. The main `r5` binary is the **custom node relayer**, and inside the `/build/bin` folder you will find:

| File | Description |
|-|-|
| cliwallet | A CLI wallet module that can be used to generate wallet addresses, send transactions and more. |
| node | The core binary of the R5 node. Can be executed separately and accepts flags, however, we do recommend starting the node via the provided relayer. |
| proxy | A native SSL proxy for RPC operators. Accepts the `--gencert` flag to generate self-signed certificates, and can be run to serve requests via the SSL port.
| r5console | R5's custom CLI console module, with an user-friendly interface, even for beginners. |

For more information about each specific tool, please visit R5's documents library at https://docs.r5.network/.

#### The `r5` Relayer

Inside the `/build` folder, you will find the main node binary, named `r5` (or `r5.exe` if you're on Windows). This is the recommended entry-point for your R5 node. It simplifies the process of starting the node and consolidates the old `Geth` flags into a much easier to understand and operate set of initialisation flags.

| Flag | Parameters | Default | Description |
|-|-|-|-|
| `-h` or `--help` | n/a | n/a | Displays a list of supported flags and information about each one. **This flag must be used alone.** |
| `--network` | `mainnet` `devnet` `testnet` `local` | `mainnet` | Used to define the network used to initialise the node. |
| `--rpc` | n/a | `disabled` | Enables the node's RPC service, opening `http`, `ws`, and `graphql` endpoints, with `web3`, `eth`, `r5`, and `net` nameservers. **It is strongly recommended using the provided proxy to serve only `HTTPS` requests,** with rate-limitting and CORS configuration. |
| `--node` | `archive` `full` `light` | `full` | Determines the type of node to run. |
| `--miner` | `coinbase` `threads` | `disabled` | Enables mining on the node and defines the `coinbase` and the amount of `threads` to use. |
| `--jsconsole` | n/a | n/a | Opens the R5 JS Console. It requires a running node to work. **This flag must be used alone.** |

Usage example for starting a mainnet RPC archive node with mining enabled:

```bash
./r5 --network mainnet --rpc --node archive --miner coinbase=0xABC... threads=1
```

You can also configure the initialisation parameters by creating or modifying a `node.ini` file. You can use `default` to populate any parameter you don't want to change. An example of how you can structure your `node.ini` file:

```bash
[R5 Node Relayer]
network=mainnet
rpc=true
mode=default
miner=true
miner.coinbase=0xABC...
miner.threads=1
genesis=default
config=default
```

The folder structure of your build should contain the following subdirectories:

| Subdirectory | Description |
|-|-|
| `/bin` | All the main binaries and their dependencies - configuration files and other subdirectories, for example - including tools. |
| `/config` | Configuration files for R5 networks. |
| `/json` | Genesis files for `testnet`,  `devnet`, and `local` networks. |

Other subdirectories will be created once the node starts, including the main data storage folder.

### Core Binary Build

If you prefer, you can build only the core protocol binary to run your node. To that end, you can run:

```bash
python3 build.py --coreonly
```

Once the build has finished, you will see your binary inside the `/build` folder.

This version of the build doesn't use R5's node relayer, so you will need to use it with the standard `Geth` flag structure, and **it does not offer ready-support for running `testnet` or `devnet` nodes.**

Unless you are very familiar with `Geth`, **we do not recommend using this method** to build your node binaries.

You may face some compatibility issues with some of `Geth`'s standard flags and commands.

---

Fore more information, guides, and tutorials, please visit R5's documents library at https://docs.r5.network.

**This repository is constantly changing, and some of the information above may be out of date.**