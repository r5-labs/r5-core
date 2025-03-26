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

// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.19;

contract R5ZKNet {
    struct InternalAccount {
        uint256 balance; // Stores the balance of the internal account
        bool exists; // Flag to check if an internal account exists
        uint256 nonce; // Used for replay protection
    }

    mapping(bytes32 => InternalAccount) private accounts; // Maps internal addresses to account data
    mapping(address => bytes32) private walletToInternal; // Maps user wallets to their internal addresses

    // Helper Functions
    function toHexString(bytes32 data) internal pure returns (string memory) {
        bytes memory alphabet = "0123456789abcdef";
        bytes memory str = new bytes(64);

        for (uint256 i = 0; i < 32; i++) {
            str[i * 2] = alphabet[uint8(data[i] >> 4)];
            str[1 + i * 2] = alphabet[uint8(data[i] & 0x0f)];
        }

        return string(str);
    }

    function fromHexString(string memory hexString) internal pure returns (bytes32) {
        require(bytes(hexString).length == 64, "Invalid hex length");

        bytes32 result;
        bytes memory bytesArray = bytes(hexString);
        for (uint256 i = 0; i < 32; i++) {
            uint8 high = _charToByte(bytesArray[i * 2]);
            uint8 low = _charToByte(bytesArray[i * 2 + 1]);
            result |= bytes32(uint256(high * 16 + low) << (248 - i * 8));
        }
        return result;
    }

    function _charToByte(bytes1 char) internal pure returns (uint8) {
        if (uint8(char) >= 48 && uint8(char) <= 57) return uint8(char) - 48; // '0'-'9'
        if (uint8(char) >= 97 && uint8(char) <= 102) return uint8(char) - 87; // 'a'-'f'
        if (uint8(char) >= 65 && uint8(char) <= 70) return uint8(char) - 55; // 'A'-'F'
        revert("Invalid hex character");
    }

    function slice(string memory str, uint256 start, uint256 end) internal pure returns (string memory) {
        bytes memory strBytes = bytes(str);
        require(end > start && end <= strBytes.length, "Invalid slice range");

        bytes memory result = new bytes(end - start);
        for (uint256 i = start; i < end; i++) {
            result[i - start] = strBytes[i];
        }
        return string(result);
    }

    // Function to generate a new internal account.
    // Each wallet is allowed a single internal account.
    // User needs to provide own salt to randomize the account address.
    function r5_accountCreate(string memory userSalt) external {
        require(walletToInternal[msg.sender] == bytes32(0), "Account already exists");

        bytes32 internalAddress = keccak256(
            abi.encodePacked(msg.sender, block.timestamp, block.difficulty, bytes(userSalt))
        );

        accounts[internalAddress] = InternalAccount({balance: 0, exists: true, nonce: 0});
        walletToInternal[msg.sender] = internalAddress;
    }

    // Function to deposit funds from the main network to ZKNet.
    // The user must send the specified amount in the transaction.
    function r5_balanceDeposit(uint256 amount) external payable {
        bytes32 internalAddress = walletToInternal[msg.sender];
        require(accounts[internalAddress].exists, "No internal account found");
        require(msg.value == amount, "Sent value must match the specified amount");
        require(amount > 0, "Deposit amount must be greater than zero");
        accounts[internalAddress].balance += msg.value;
    }

    // Function to transfer funds from one ZKNet wallet to another.
    function r5_balanceTransferInternal(string memory destinationZK, uint256 amount) external {
        bytes32 senderAddress = walletToInternal[msg.sender];
        require(accounts[senderAddress].exists, "No internal account found");
        require(bytes(destinationZK).length == 66, "Invalid zk address length"); // 'zk' + 64 hex chars
        bytes32 destination = fromHexString(slice(destinationZK, 2, 66));
        require(accounts[destination].exists, "Recipient account does not exist");
        require(accounts[senderAddress].balance >= amount, "Insufficient balance");

        // Update balances
        accounts[senderAddress].balance -= amount;
        accounts[destination].balance += amount;

        // Automatically increment nonce
        accounts[senderAddress].nonce++;
    }

    // Function to transfer funds from a ZKNet wallet to a main network wallet.
    function r5_balanceTransferExternal(address payable recipient, uint256 amount) external {
        bytes32 senderAddress = walletToInternal[msg.sender];
        require(accounts[senderAddress].exists, "No internal account found");
        require(accounts[senderAddress].balance >= amount, "Insufficient balance");

        // Update balances
        accounts[senderAddress].balance -= amount;
        recipient.transfer(amount);

        // Automatically increment nonce
        accounts[senderAddress].nonce++;
    }

    // Function that returns the balance of a ZKNet wallet.
    function r5_balanceCheck() external view returns (uint256) {
        bytes32 internalAddress = walletToInternal[msg.sender];
        require(accounts[internalAddress].exists, "No internal account found");
        return accounts[internalAddress].balance;
    }

    // Destroys internal address and sends any remaining funds to an external wallet.
    function r5_accountDestroy(address payable recipient) external {
        bytes32 internalAddress = walletToInternal[msg.sender];
        require(accounts[internalAddress].exists, "No internal account found");

        uint256 remainingBalance = accounts[internalAddress].balance;
    
        // First delete mappings (gas refund optimization)
        delete walletToInternal[msg.sender];
        delete accounts[internalAddress];

        if (remainingBalance > 0) {
            recipient.transfer(remainingBalance);
        }
    }

    // Function to resolve and return the internal address associated with the caller's wallet.
    function r5_accountResolve() external view returns (string memory) {
        bytes32 internalAddress = walletToInternal[msg.sender];
        require(internalAddress != bytes32(0), "No internal account found");
    
        return string(abi.encodePacked("zk", toHexString(internalAddress)));
    }
}
