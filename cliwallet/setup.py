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

from setuptools import setup

setup(
    name='r5wallet',
    version='1.0.0',
    description='A CLI wallet for the R5 Network',
    author='Paulo Baronceli',
    author_email='support@r5.network',
    py_modules=['main'],
    install_requires=[
        'web3',
        'ecdsa',
        'cryptography',
        'pyinstaller',
    ],
    entry_points={
        'console_scripts': [
            'r5wallet = main:main',
        ],
    },
)