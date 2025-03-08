from setuptools import setup, find_packages

setup(
    name="geth_wallet",
    version="0.1",
    packages=find_packages(),
    install_requires=[
        "pyqt6",
        "web3",
    ],
    entry_points={
        "console_scripts": [
            "geth-wallet=main:main",
        ],
    },
)
