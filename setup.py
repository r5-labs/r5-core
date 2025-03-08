#!/usr/bin/env python3
# Copyright 2025 R5
# This file is part of the R5 Core library.
#
# This software is provided "as is", without warranty of any kind,
# express or implied, including but not limited to the warranties
# of merchantability, fitness for a particular purpose and
# noninfringement. In no event shall the authors or copyright
# holders be liable for any claim, damages, or other liability,
# whether in an action of contract, tort or otherwise, arising
# from, out of or in connection with the software or the use or
# other dealings in the software.
#
# Author: ZNX

from setuptools import setup, find_packages

setup(
    name='r5-tools',
    version='1.0.0',
    description='CLI tools for the R5 Network: Wallet, Relayer, and SSL Proxy',
    author='Your Name or R5 Core Team',
    author_email='support@r5.network',
    packages=find_packages(),
    install_requires=[
        'web3',
        'ecdsa',
        'cryptography',
        'pyinstaller',
        'limits',
    ],
    entry_points={
        'console_scripts': [
            'r5wallet = cliwallet.main:main',
            'r5 = relayer.main:main',
            'r5-proxy = proxy.main:main',
        ],
    },
    classifiers=[
        "Programming Language :: Python :: 3",
        "Operating System :: OS Independent",
    ],
    python_requires=">=3.6",
)
