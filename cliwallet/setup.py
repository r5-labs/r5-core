from setuptools import setup

setup(
    name='r5wallet',
    version='1.0.0',
    description='A CLI wallet for the R5 Network',
    author='Paulo Baronceli',
    author_email='contact@r5.network',
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