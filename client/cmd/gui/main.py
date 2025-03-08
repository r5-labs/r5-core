import sys
import subprocess
import json
from PyQt6.QtWidgets import (QApplication, QMainWindow, QAction, QVBoxLayout, QPushButton, QLabel, QLineEdit, QTextEdit, QFileDialog, QHBoxLayout, QMenuBar, QStatusBar, QWidget)
from web3 import Web3

class WalletGUI(QMainWindow):
    def __init__(self):
        super().__init__()
        self.rpc_url = "http://127.0.0.1:8545"
        self.web3 = Web3(Web3.HTTPProvider(self.rpc_url))
        self.initUI()

    def initUI(self):
        self.setWindowTitle("Geth Wallet")
        self.setGeometry(100, 100, 600, 400)

        # Menu Bar
        menu_bar = self.menuBar()
        file_menu = menu_bar.addMenu("File")
        settings_menu = menu_bar.addMenu("Settings")

        exit_action = QAction("Exit", self)
        exit_action.triggered.connect(self.close)
        file_menu.addAction(exit_action)

        settings_action = QAction("Configure RPC", self)
        settings_action.triggered.connect(self.show_rpc_settings)
        settings_menu.addAction(settings_action)

        # Central Widget
        central_widget = QWidget()
        self.setCentralWidget(central_widget)

        layout = QVBoxLayout()

        # Wallet Controls
        self.wallet_info = QTextEdit()
        self.wallet_info.setReadOnly(True)

        create_wallet_btn = QPushButton("Create Wallet")
        create_wallet_btn.clicked.connect(self.create_wallet)

        send_tx_btn = QPushButton("Send Transaction")
        send_tx_btn.clicked.connect(self.send_transaction)

        geth_console_btn = QPushButton("Open Geth Console")
        geth_console_btn.clicked.connect(self.open_geth_console)

        # Layout Management
        btn_layout = QHBoxLayout()
        btn_layout.addWidget(create_wallet_btn)
        btn_layout.addWidget(send_tx_btn)
        btn_layout.addWidget(geth_console_btn)

        layout.addWidget(self.wallet_info)
        layout.addLayout(btn_layout)

        central_widget.setLayout(layout)

        # Status Bar
        self.status_bar = QStatusBar()
        self.setStatusBar(self.status_bar)

    def show_rpc_settings(self):
        self.rpc_input = QLineEdit(self.rpc_url, self)
        self.rpc_input.returnPressed.connect(self.update_rpc_url)
        self.status_bar.showMessage("Enter new RPC URL and press Enter")

    def update_rpc_url(self):
        self.rpc_url = self.rpc_input.text()
        self.web3 = Web3(Web3.HTTPProvider(self.rpc_url))
        self.status_bar.showMessage(f"RPC Updated: {self.rpc_url}")

    def create_wallet(self):
        account = self.web3.eth.account.create()
        wallet_data = {
            "address": account.address,
            "private_key": account.key.hex()
        }
        self.wallet_info.setText(json.dumps(wallet_data, indent=4))
        
        file_name, _ = QFileDialog.getSaveFileName(self, "Save Wallet", "wallet.json", "JSON Files (*.json)")
        if file_name:
            with open(file_name, 'w') as f:
                json.dump(wallet_data, f, indent=4)

    def send_transaction(self):
        self.wallet_info.setText("Send Transaction functionality not yet implemented.")

    def open_geth_console(self):
        try:
            subprocess.Popen(["geth", "attach", self.rpc_url], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        except FileNotFoundError:
            self.wallet_info.setText("Error: geth binary not found. Ensure it's installed and in PATH.")

if __name__ == "__main__":
    app = QApplication(sys.argv)
    gui = WalletGUI()
    gui.show()
    sys.exit(app.exec_())
