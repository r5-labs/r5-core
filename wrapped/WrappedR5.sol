// SPDX-License-Identifier: GPL-3.0-or-later
pragma solidity ^0.8.29;

contract WrappedR5 {
    // ERC20 metadata
    string public constant name     = "Wrapped R5";
    string public constant symbol   = "WR5";
    uint8  public constant decimals = 18;

    // Events with standard parameter names
    event Approval(address indexed owner, address indexed spender, uint256 value);
    event Transfer(address indexed from, address indexed to, uint256 value);
    event Deposit(address indexed to, uint256 amount);
    event Withdrawal(address indexed from, uint256 amount);

    // Balances and allowances
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    // Allow contract to receive native R5
    receive() external payable {
        deposit();
    }
    fallback() external payable {
        deposit();
    }

    /// @notice Wrap native R5 into WR5
    function deposit() public payable {
        balanceOf[msg.sender] += msg.value;
        emit Deposit(msg.sender, msg.value);
    }

    /// @notice Unwrap WR5 into native R5
    function withdraw(uint256 amount) public {
        require(balanceOf[msg.sender] >= amount, "WrappedR5: insufficient balance");
        balanceOf[msg.sender] -= amount;
        payable(msg.sender).transfer(amount);
        emit Withdrawal(msg.sender, amount);
    }

    /// @return Total WR5 in circulation
    function totalSupply() public view returns (uint256) {
        return address(this).balance;
    }

    /// @notice Approve `spender` to spend WR5
    function approve(address spender, uint256 amount) public returns (bool) {
        allowance[msg.sender][spender] = amount;
        emit Approval(msg.sender, spender, amount);
        return true;
    }

    /// @notice Transfer WR5 to `to`
    function transfer(address to, uint256 amount) public returns (bool) {
        return transferFrom(msg.sender, to, amount);
    }

    /// @notice Transfer WR5 on behalf of `from`
    function transferFrom(address from, address to, uint256 amount) public returns (bool) {
        require(balanceOf[from] >= amount, "WrappedR5: insufficient balance");

        if (from != msg.sender && allowance[from][msg.sender] != type(uint256).max) {
            require(allowance[from][msg.sender] >= amount, "WrappedR5: allowance exceeded");
            allowance[from][msg.sender] -= amount;
        }

        balanceOf[from] -= amount;
        balanceOf[to] += amount;
        emit Transfer(from, to, amount);
        return true;
    }
}
