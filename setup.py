from setuptools import setup, find_packages

setup(
    name='r5-tools',
    version='1.0.0',
    description='CLI tools for the R5 Network: Wallet and Relayer',
    author='Your Name or R5 Core Team',
    author_email='support@r5.network',
    packages=find_packages(),
    install_requires=[
        'web3',
        'ecdsa',
        'cryptography',
        'pyinstaller',
    ],
    entry_points={
        'console_scripts': [
            'r5wallet = cliwallet.main:main',
            'r5 = relayer.main:main',
        ],
    },
    classifiers=[
        "Programming Language :: Python :: 3",
        "Operating System :: OS Independent",
    ],
    python_requires=">=3.6",
)
