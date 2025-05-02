// SPDX-License-Identifier: MIT
/*
* TODO: Add comments; Implement slashing?; Decentralise (no owner)
*/
pragma solidity ^0.8.19;

contract R5SubnetRegistry {
    struct Subnet {
        address owner;
        string endpoint;
        uint256 stake;
        uint256 lastUpdated;
    }

    mapping(uint256 => Subnet) public subnets;
    mapping(address => uint256[]) public ownedSubnets;

    uint256 public constant MINIMUM_STAKE = 1 ether;
    uint256 public totalSubnets;

    event SubnetRegistered(uint256 indexed subnetId, address indexed owner, string endpoint, uint256 stake);
    event SubnetUpdated(uint256 indexed subnetId, string newEndpoint);
    event StakeIncreased(uint256 indexed subnetId, uint256 addedAmount);
    event StakeWithdrawn(uint256 indexed subnetId, uint256 amount);

    modifier onlyOwner(uint256 subnetId) {
        require(subnets[subnetId].owner == msg.sender, "Not owner");
        _;
    }

    function registerSubnet(uint256 subnetId, string calldata endpoint) external payable {
        require(subnets[subnetId].owner == address(0), "Already registered");
        require(msg.value >= MINIMUM_STAKE, "Insufficient stake");

        subnets[subnetId] = Subnet({
            owner: msg.sender,
            endpoint: endpoint,
            stake: msg.value,
            lastUpdated: block.timestamp
        });

        ownedSubnets[msg.sender].push(subnetId);
        totalSubnets++;

        emit SubnetRegistered(subnetId, msg.sender, endpoint, msg.value);
    }

    function updateEndpoint(uint256 subnetId, string calldata newEndpoint) external onlyOwner(subnetId) {
        subnets[subnetId].endpoint = newEndpoint;
        subnets[subnetId].lastUpdated = block.timestamp;

        emit SubnetUpdated(subnetId, newEndpoint);
    }

    function addStake(uint256 subnetId) external payable onlyOwner(subnetId) {
        require(msg.value > 0, "No ETH sent");
        subnets[subnetId].stake += msg.value;

        emit StakeIncreased(subnetId, msg.value);
    }

    function withdrawStake(uint256 subnetId, uint256 amount) external onlyOwner(subnetId) {
        Subnet storage subnet = subnets[subnetId];
        require(amount > 0 && amount <= subnet.stake, "Invalid amount");

        subnet.stake -= amount;
        payable(msg.sender).transfer(amount);

        emit StakeWithdrawn(subnetId, amount);
    }

    function resolveEndpoint(uint256 subnetId) external view returns (string memory) {
        require(subnets[subnetId].owner != address(0), "Not found");
        return subnets[subnetId].endpoint;
    }

    function getSubnetsByOwner(address owner) external view returns (uint256[] memory) {
        return ownedSubnets[owner];
    }
}
