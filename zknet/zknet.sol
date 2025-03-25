// Copyright 2025 R5 Labs
// This file is part of the R5 Core library.
//
// This software is provided "as is", without warranty of any kind,
// express or implied, including but not limited to the warranties
// of merchantability, fitness for a particular purpose and
// noninfringement. In no event shall the authors or copyright
// holders be liable for any claim, damages, or other liability,
// whether in an action of contract, tort or otherwise, arising
// from, out of or in connection with the software or the use or
// other dealings in the software.

// CODE BEING REVIEWED, NOT READY FOR DEPLOYMENT

// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.19;

contract R5ZKNet {
    struct InternalAccount {
        uint256 balance; // Stores the balance of the internal account
        bool exists; // Flag to check if an internal account exists
    }

    mapping(bytes32 => InternalAccount) private accounts; // Maps internal addresses to account data
    mapping(address => bytes32) private walletToInternal; // Maps user wallets to their internal addresses
    mapping(bytes32 => Transaction) private txnHashes; // Store transaction hashes and details

    bool public networkOnline = true; // Tracks whether R5ZKNet is active
    uint256 public pauseTimestamp; // Timestamp of when the network was paused
    address private admin; // Declares the "admin" address

    // Emits an event when the network is turned online/offline
    event NetworkStatusChanged(bool online);

    // Transaction struct for txnHashes mapping
    struct Transaction {
        bytes32 sender;
        bytes32 recipient;
        uint256 amount;
    }

    // Restricts access to admin-only functions
    modifier onlyAdmin() {
        require(
            msg.sender == admin,
            "Only contract owner can perform this action"
        );
        _;
    }

    // Ensures functions can only be executed when the network is online
    modifier whenOnline() {
        require(networkOnline, "ZKNet is offline");
        _;
    }

    constructor() {
        admin = msg.sender; // Sets contract deployer as owner
    }

    // F1: r5_accountCreate()
    // Function to generate a new internal account.
    // Each wallet is allowed a single internal account.
    // Account generation is not deterministic, therefore cannot be recovered if lost.
    function r5_accountCreate() external whenOnline {
        require(
            walletToInternal[msg.sender] == bytes32(0),
            "Account already exists"
        );

        // Generate a unique internal address using a hash of the user address and randomness
        bytes32 internalAddress = keccak256(
            abi.encodePacked(msg.sender, block.timestamp, block.difficulty)
        );

        accounts[internalAddress] = InternalAccount({balance: 0, exists: true});
        walletToInternal[msg.sender] = internalAddress;
    }

    // F2: r5_balanceDeposit()
    // Function to deposit funds from the main network to ZKNet.
    // The user must send the specified amount in the transaction.
    function r5_balanceDeposit(uint256 amount) external payable whenOnline {
        // Ensure the caller has an internal account
        bytes32 internalAddress = walletToInternal[msg.sender];
        require(accounts[internalAddress].exists, "No internal account found");

        // Ensure the amount sent matches the amount specified in the parameter
        require(
            msg.value == amount,
            "Sent value must match the specified amount"
        );
        require(amount > 0, "Deposit amount must be greater than zero");

        // Update the internal account balance
        accounts[internalAddress].balance += msg.value;
    }

    // F3: r5_balanceTransferInternal()
    // Function to transfer funds from one ZKNet wallet to another.
    function r5_balanceTransferInternal(
        bytes32 destination,
        uint256 amount
    ) external whenOnline {
        bytes32 senderAddress = walletToInternal[msg.sender];
        require(accounts[senderAddress].exists, "No internal account found");
        require(
            accounts[destination].exists,
            "Recipient account does not exist"
        );
        require(
            accounts[senderAddress].balance >= amount,
            "Insufficient balance"
        );

        // Deduct from sender and add to receiver
        accounts[senderAddress].balance -= amount;
        accounts[destination].balance += amount;

        // Storing the transaction details for later verification
        bytes32 txnHash = keccak256(
            abi.encodePacked(
                senderAddress,
                destination,
                amount,
                block.timestamp
            )
        );
        txnHashes[txnHash] = Transaction(senderAddress, destination, amount);
    }

    // F4: r5_balanceTransferExternal()
    // Function to transfer funds from a ZKNet wallet to a main network wallet.
    // Gas is deducted from the msg.sender's wallet.
    function r5_balanceTransferExternal(
        address payable recipient,
        uint256 amount
    ) external whenOnline {
        bytes32 senderAddress = walletToInternal[msg.sender];
        require(accounts[senderAddress].exists, "No internal account found");
        require(
            accounts[senderAddress].balance >= amount,
            "Insufficient balance"
        );

        // Deduct from sender before transferring
        accounts[senderAddress].balance -= amount;

        // Transfer funds
        recipient.transfer(amount);
    }

    // F5: r5_balanceCheck()
    // Function that returns the balance of a ZKNet wallet.
    // Can only be called by the ZKNet wallet owner.
    function r5_balanceCheck() external view returns (uint256) {
        bytes32 internalAddress = walletToInternal[msg.sender];
        require(accounts[internalAddress].exists, "No internal account found");
        return accounts[internalAddress].balance;
    }

    // F6: r5_txnHashCheck
    // Function to check if a transaction has been confirmed or not.
    // Only transaction sender and destination can call this function.
    function r5_txnHashCheck(
        bytes32 txnHash
    ) external view returns (bool exists, uint256 amount) {
        bytes32 internalAddress = walletToInternal[msg.sender];
        require(accounts[internalAddress].exists, "No internal account found");

        // Ensure only sender or recipient of the transaction can check
        Transaction memory txn = txnHashes[txnHash];
        require(
            internalAddress == txn.sender || internalAddress == txn.recipient,
            "Unauthorized access"
        );

        return (true, txn.amount);
    }

    // F7: r5_accountDestroy()
    // Destroys internal address and sends any remaining funds to an external wallet.
    // The internal address will be lost forever, and the user will have to create a new
    // internal account address if they wish to use the ZKNet protocol again with the same
    // external wallet address.
    function r5_accountDestroy(address payable recipient) external whenOnline {
        bytes32 internalAddress = walletToInternal[msg.sender];
        require(accounts[internalAddress].exists, "No internal account found");

        uint256 remainingBalance = accounts[internalAddress].balance;

        // Delete internal account and mapping entry
        delete accounts[internalAddress];
        delete walletToInternal[msg.sender];

        if (remainingBalance > 0) {
            recipient.transfer(remainingBalance);
        }
    }

    // F8: r5_accountResolve()
    // Function to resolve and return the internal address associated with the caller's wallet.
    function r5_accountResolve()
        external
        view
        returns (bytes32 internalAddress)
    {
        internalAddress = walletToInternal[msg.sender];
        require(internalAddress != bytes32(0), "No internal account found");
        return internalAddress; // Returns the internal account address associated with the caller's wallet
    }

    // F9: admin_setNetworkStatus
    // Toggles between "Online" and "Offline" status. This function is meant to be used
    // only in emergencies and stops all internal functions of the smart contract.
    function admin_setNetworkStatus(bool online) external onlyAdmin {
        networkOnline = online;
        if (!online) {
            pauseTimestamp = block.timestamp;
        }

        emit NetworkStatusChanged(online);
    }

    // F10: admin_emergencyWithdraw
    // Meant to be used only in emergencies. It allows the contract owner to withdraw the contract
    // funds to mitigate any potential unforeseen exploits. For security and transparency, it can
    // only be called 14 days after the network has been paused.
    function admin_emergencyWithdraw(
        address payable recipient,
        uint256 amount
    ) external onlyAdmin {
        require(!networkOnline, "Network must be paused first");
        require(
            block.timestamp >= pauseTimestamp + 14 days,
            "Emergency withdrawal not allowed yet"
        );

        recipient.transfer(amount);
    }
}
