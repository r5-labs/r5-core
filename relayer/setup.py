#!/usr/bin/env python3# Copyright 2025 R5
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

from setuptools import setup, find_packages

setup(
    name="r5-relayer",
    version="0.1.0",
    description="R5 Node Relayer - Simplified entry point for starting an R5 node",
    author="R5 Core Team",
    author_email="support@r5.network",
    url="https://github.com/r5-labs/r5-core",
    packages=find_packages(),
    entry_points={
        'console_scripts': [
            'r5 = relayer.main:main',
        ],
    },
    classifiers=[
        "Programming Language :: Python :: 3",
        "Operating System :: OS Independent",
    ],
    python_requires=">=3.6",
)
