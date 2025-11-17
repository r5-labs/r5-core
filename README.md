# R5 Core

![R5 Logo](img/r5.png)

R5 revisits the proof‑of‑work consensus mechanism to create a
high‑performance, secure blockchain. It is capable of processing over 1,000
transactions per second while leveraging battle‑tested architecture and
privacy features. This repository contains the core implementation of the R5
Protocol, including the R5 node relayer and all accompanying SDK tools.

**This repository is constantly evolving; some information above may change
over time. Please refer to the latest documentation for up‑to‑date details.**

## Hardware Requirements

Please note that these are the **minimum** hardware requirements for running
each one of the node types specified. For more information, including
**recommended** harware specs, please visit https://docs.r5.network/for-
developers/hardware-requirements.

Description | RPC Nodes* | Archive Nodes | Full Nodes | Light Nodes  
---|---|---|---|---  
CPU Cores | 12 | 12 | 4 | 2  
RAM | 32 GB | 10 GB | 8 GB | 2 GB  
Storage | NVMe >240GB* | SSD 1 TB | SSD 240 GB | HDD 1GB  
Network | Stable 125MB/s | Stable 125MB/s | Stable 100MB/s | Stable 10MB/s  
  
_RPC Node storage requirements will vary based on the type of underlying node
(Archive or Full)._ _Minimum requirements for RPC nodes are recommendations
for Public RPC server implementations. Private RPC nodes will have the same
minimum hardware requirements as regular node implementations, with
recommended specs subject to your own usage requirements._

### What happens if I bypass the minimum hardware requirements?

There are hardcoded checks that will prevent the node from starting, but if
you bypass those, there is a good chance that your node will either be
unstable - best case scenario - or it will try to sync, but due to not being
able to "keep up" with the rest of the network, it will fork and stop. The
**Light Node** hardware requirements are very basic, and meant to support even
"outdated" and "old" devices, and you can always opt to run a light node if
your machine doesn't meet the minimum requirements for a Full or an Archive
node.

## Pre-Requisites

You will need to have `Python 3`, `Golang 1.19`, and a `C` compiler (such as
`gcc`, for example) to build the core binary, and additional Python
dependencies when building the full set of protocol tools.

If you prefer, the `install.py` script installs all required dependencies
automatically. If you are running Windows, you might need to install a C
compiler separately, even after running the install script.

You can run:

    
    
    python3 install.py
    

Or, if on Windows:

    
    
    python install.py
    

### !! IMPORTANT !!

If your system restricts system-wide package installing - eg. Ubuntu Desktop
24.04 - you may have to take some extra steps to install all dependencies
before building the package.

The `build.py` script will automatically detect most of such limitations and
do all the work for you, however, if after installing all dependencies you
face issues with building the code from source, please follow the steps below.

First, make sure you have `golang-1.19/stable` installed. If you're using
Ubuntu, you can install it via `snap` with the following command:

    
    
    sudo snap install go --channel=1.19/stable --classic
    

To install the `python` dependencies, you will need to create a virtual
environment to separate the program packages from your system packages. First,
install `v-env` with the following command:

    
    
    sudo apt install python3.12-venv
    

Then, create the virtual environment (named `r5-venv` in this example):

    
    
    python3 -m venv r5-venv
    

Enter the virtual environment you have just created with:

    
    
    source r5-venv/bin/activate
    

Now you can install the python dependencies with:

    
    
    pip install .
    

And follow the instructions below to build the binaries. Please be aware that
you still need to have your virtual environment active to be able to build the
SDK tools, otherwise, you will only be able to build the main node binary
using the `build.py` script with the `--coreonly` flag.

## Building R5

You can build the binaries by running the `build.py` script. To compile only
the core node binary, run the script with the `--coreonly` flag. If no flag is
provided, the script will build the full R5 Protocol with all its tools.

Run on Linux/macOS:

    
    
    python3 build.py
    

Or on Windows:

    
    
    python build.py
    

### Full Protocol Build

This build compiles all the tools included with the R5 Protocol, giving you a
fully functioning node in your `/build` folder. The main `r5` binary is the
custom node relayer, and inside the `/build/bin` folder you will find:

File | Description  
---|---  
cliwallet | A CLI wallet module that can generate wallet addresses, send transactions, and perform other wallet functions.  
node | The core binary of the R5 node. It can be executed separately and accepts flags, though we recommend starting the node via the provided relayer.  
proxy | A native SSL proxy for RPC operators. It accepts the --gencert flag to generate self-signed certificates and can serve requests via the SSL port.  
console | R5's custom CLI console module, offering a user-friendly interface even for beginners.  
scdev | SCdev is a powerful smart contract interface that allows you to compile, deploy, and interact with smart contracts, as well as manage accounts.  
  
For more information about each tool, please visit R5's documentation library
at https://docs.r5.network.

#### The r5 Relayer

Inside the `/build` folder, you will find the main node binary, named `r5` (or
`r5.exe` on Windows). This is the recommended entry point for your R5 node.
The relayer simplifies node startup by consolidating the traditional `Geth`
flag structure into an easier-to-use set of initialisation options.

Flag | Parameters | Information  
---|---|---  
`--bypass` |  | Used to bypass commands directly to your `node` binary for advanced configuration. **Use with caution! This flag must be used alone.**  
`--cliwallet` |  | Starts the CLI Wallet. **This flag must be used alone.**  
`-h` or `--help` |  | Prints the list of supported flags and parameters. **This flag must be used alone.**  
`--jsconsole` |  | Starts the JS Console. **This flag must be used alone.**  
`--miner` | `coinbase` |   
`threads` | Starts the node with mining enabled. If you don't set a custom |   
`coinbase` it will burn the mining rewards by default, and if you want active |  |   
mining directly on the node's CPU, you need to set `threads` > 0\. Disabled by |  |   
default. |  |   
`--network` | `mainnet` |   
`testnet` |  |   
`devnet` |  |   
`local` | Defines the network you want to connect your node to. Local networks |   
use `ChainId` `13512` by default. Defaults to `mainnet` . |  |   
`--mode` | `archive` |   
`full` |  |   
`light` | Defines the type of node to start. Defaults to `full`. |   
`--proxy` | `gencert` | Starts the SSL Proxy server. Note that it requires a  
running node to work. By default, it forwards incoming requests on port `443` |  |   
to port `8545`. You can use the `gencert` flag to generate self-signed |  |   
certificates. **This flag must be used alone.** |  |   
`--r5console` |  | Starts the built-in R5 Console. **This flag must be used alone.**  
`--rpc` |  | Enables the node's RPC service. It enables the `http`, `ws`, and `graphql` services, and opens the `web3`, `eth`, `r5`, and `net` API endpoints by default. `http` requests are served on port `8545`. **For production environments, it is recommended to be used alongside the SSL Proxy provided.** Disabled by default.  
  
**Usage Example:** Starting a mainnet RPC archive node with mining enabled:

    
    
    ./r5 --network mainnet --rpc --node archive --miner coinbase=0xABC... threads=1
    

## Configuration File "node.ini"

For faster and more consistent deployment, you can pre-configure a `node.ini`
file and place it in the same folder as your r5 binary. This file acts as a
preset for node initialisation settings. Any flags provided during startup
will override the node.ini settings.

### Example node.ini File

    
    
    [R5 Node Relayer]
    network = mainnet
    rpc = true
    mode = default
    miner = true
    miner.coinbase = 0xABC...
    miner.threads = 1
    genesis = default
    config = default
    

## Folder Structure

The `/build` folder should contain the following subdirectories:

Subdirectory | Description  
---|---  
`/bin` | Contains the main binaries and their dependencies, including configuration files and tools.  
`/config` | Contains configuration files for R5 networks.  
`/genesis` | Contains genesis files for testnet, devnet, and local networks.  
  
Additional directories, such as the main data storage folder, will be created
when the node starts.

## Core Binary Build

If you prefer to build only the core protocol binary, run:

    
    
    python3 build.py --coreonly
    

After building, the binary will be located in the /build folder. Note that
this build does not utilise the r5 node relayer and will require the standard
Geth flag structure. It does not offer ready-support for running testnet or
devnet nodes, so we do not recommend this method unless you are very familiar
with Geth.

## Running Your Node

After building, start your node using the r5 Relayer with the desired
configuration. For example, to start a mainnet RPC archive node with mining
enabled, run:

    
    
    ./r5 --network mainnet --rpc --node archive --miner coinbase=0xABC... threads=1
    

You may also configure initialisation parameters using the node.ini file for a
more streamlined deployment.

## SDK Tools

The R5 SDK suite provides a collection of tools to enhance node deployment and
management:

  * **SSL Proxy:** Provides native SSL support for RPC nodes and includes a built-in tool for generating self‑signed certificates.
  * **R5 Console:** A user-friendly CLI console for node operators.
  * **JS Console:** Grants direct access to the node’s JavaScript runtime and full control over all RPC API namespaces.
  * **CLI Wallet:** A command-line wallet for executing transactions and managing multiple wallet addresses.
  * **SCdev:** A tool for deploying and interacting with smart contracts on the R5 Network (currently under development).

For further details and tutorials, please visit [R5
Documentation](https://docs.r5.network).

