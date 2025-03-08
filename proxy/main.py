#!/usr/bin/env python3
"""
R5 SSL Proxy for Node RPC with Robust Rate Limiting and Logging

This proxy:
  - Reads settings from proxy.ini (creates one with defaults if missing)
  - Optionally generates a CA-signed self-signed certificate (--gencert)
  - Listens for HTTPS connections (default listen port: 443) and forwards them to the target host/port (default: localhost:8545)
  - Adds CORS headers to both requests and responses
  - Enforces per‑IP rate limiting using a fixed-window strategy
"""

import argparse
import configparser
import logging
import os
import sys
import ssl
import http.server
import http.client
import threading
import time
from urllib.parse import urlparse
from datetime import datetime, timedelta, timezone

# Import limits for rate limiting.
try:
    from limits import RateLimitItemPerMinute
    from limits.strategies import FixedWindowRateLimiter
    from limits.storage import MemoryStorage
except ImportError:
    sys.exit("Please install the 'limits' package via pip (pip install limits)")

# Import cryptography for certificate generation.
try:
    from cryptography import x509
    from cryptography.x509.oid import NameOID
    from cryptography.hazmat.primitives import hashes, serialization
    from cryptography.hazmat.primitives.asymmetric import rsa
    from cryptography.hazmat.backends import default_backend
except ImportError:
    sys.exit("Please install the 'cryptography' package via pip (pip install cryptography)")

# ----- Defaults for proxy.ini -----
DEFAULT_INI = """[Proxy]
destination_host = localhost
destination_port = 8545
listen_port = 443
allowed_origin = *
# Maximum allowed requests per minute per IP. 0 means unlimited.
rate_limit = 100
# Paths to SSL files (relative to current directory)
ssl_key = cert/default.key
ssl_cert = cert/default.crt
ssl_ca = cert/default.ca
"""

# ----- Logging configuration -----
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
    datefmt="%d-%m|%H:%M:%S"
)
logger = logging.getLogger(__name__)

# ----- Rate Limiter Setup -----
storage = MemoryStorage()
limiter = FixedWindowRateLimiter(storage)

def is_rate_limited(remote_addr, rate_limit_value):
    """
    Returns True if remote_addr has exceeded rate_limit_value requests per minute.
    """
    if rate_limit_value <= 0:
        return False
    limit_item = RateLimitItemPerMinute(rate_limit_value)
    # The key is the remote IP.
    if not limiter.hit(limit_item, remote_addr):
        logger.warning("Rate limit exceeded for %s", remote_addr)
        return True
    return False

# ----- Certificate Generation Using Cryptography -----
def generate_certificates(cert_dir):
    """
    Generate a CA certificate and key, then generate a proxy certificate signed by the CA.
    Writes files to:
      - cert/default.ca : CA certificate (PEM)
      - cert/default.key: Proxy private key (PEM)
      - cert/default.crt: Proxy certificate (PEM)
    """
    os.makedirs(cert_dir, exist_ok=True)

    # 1. Generate CA private key and self-signed CA certificate
    ca_key = rsa.generate_private_key(
        public_exponent=65537,
        key_size=2048,
        backend=default_backend()
    )
    ca_subject = x509.Name([
        x509.NameAttribute(NameOID.COMMON_NAME, u"R5 CA"),
    ])
    ca_cert = (
        x509.CertificateBuilder()
        .subject_name(ca_subject)
        .issuer_name(ca_subject)
        .public_key(ca_key.public_key())
        .serial_number(x509.random_serial_number())
        .not_valid_before(datetime.now(timezone.utc) - timedelta(minutes=1))
        .not_valid_after(datetime.now(timezone.utc) + timedelta(days=365*5))
        .add_extension(x509.BasicConstraints(ca=True, path_length=None), critical=True)
        .sign(ca_key, hashes.SHA256(), default_backend())
    )

    # 2. Generate proxy private key
    proxy_key = rsa.generate_private_key(
        public_exponent=65537,
        key_size=2048,
        backend=default_backend()
    )

    # 3. Generate proxy certificate signed by the CA
    proxy_subject = x509.Name([
        x509.NameAttribute(NameOID.COMMON_NAME, u"R5 SSL Proxy"),
    ])
    proxy_cert = (
        x509.CertificateBuilder()
        .subject_name(proxy_subject)
        .issuer_name(ca_subject)  # Issued by our CA
        .public_key(proxy_key.public_key())
        .serial_number(x509.random_serial_number())
        .not_valid_before(datetime.now(timezone.utc) - timedelta(minutes=1))
        .not_valid_after(datetime.now(timezone.utc) + timedelta(days=365))
        .sign(ca_key, hashes.SHA256(), default_backend())
    )

    # Write CA certificate
    ca_path = os.path.join(cert_dir, "default.ca")
    with open(ca_path, "wb") as f:
        f.write(ca_cert.public_bytes(serialization.Encoding.PEM))
    logger.info("CA certificate written to %s", ca_path)

    # Write proxy private key
    key_path = os.path.join(cert_dir, "default.key")
    with open(key_path, "wb") as f:
        f.write(proxy_key.private_bytes(
            encoding=serialization.Encoding.PEM,
            format=serialization.PrivateFormat.TraditionalOpenSSL,
            encryption_algorithm=serialization.NoEncryption()
        ))
    logger.info("Proxy private key written to %s", key_path)

    # Write proxy certificate
    cert_path = os.path.join(cert_dir, "default.crt")
    with open(cert_path, "wb") as f:
        f.write(proxy_cert.public_bytes(serialization.Encoding.PEM))
    logger.info("Proxy certificate written to %s", cert_path)

# ----- Reverse Proxy Handler with Rate Limiting and CORS -----
class ProxyHTTPRequestHandler(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def do_REQUEST(self):
        remote_ip = self.client_address[0]
        try:
            rate_limit_value = int(self.server.config.get("rate_limit", "0"))
        except ValueError:
            rate_limit_value = 0

        if is_rate_limited(remote_ip, rate_limit_value):
            self.send_error(429, "Too Many Requests", "Rate limit exceeded")
            return

        target_host = self.server.config["destination_host"]
        try:
            target_port = int(self.server.config["destination_port"])
        except ValueError:
            self.send_error(500, "Server Error", "Invalid destination port")
            return

        allowed_origin = self.server.config.get("allowed_origin", "*")

        # Create a connection to the destination (HTTP assumed)
        conn = http.client.HTTPConnection(target_host, target_port, timeout=10)

        # Copy request headers; add/override CORS header.
        headers = {key: val for key, val in self.headers.items()}
        headers["Access-Control-Allow-Origin"] = allowed_origin

        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length) if content_length > 0 else None

        try:
            conn.request(self.command, self.path, body=body, headers=headers)
            response = conn.getresponse()
        except Exception as e:
            logger.error("Error forwarding request: %s", e)
            self.send_error(502, "Bad Gateway", str(e))
            return

        self.send_response(response.status, response.reason)
        for key, value in response.getheaders():
            # Skip hop-by-hop headers like "transfer-encoding" if chunked.
            if key.lower() == "transfer-encoding" and value.lower() == "chunked":
                continue
            self.send_header(key, value)
        # Always add CORS header.
        self.send_header("Access-Control-Allow-Origin", allowed_origin)
        self.end_headers()

        # Relay the response body.
        while True:
            chunk = response.read(8192)
            if not chunk:
                break
            self.wfile.write(chunk)
        conn.close()

    # Support all HTTP methods.
    do_GET = do_POST = do_PUT = do_DELETE = do_PATCH = do_OPTIONS = do_HEAD = do_REQUEST

    def log_message(self, format, *args):
        logger.info("%s - - [%s] %s",
                    self.address_string(),
                    self.log_date_time_string(),
                    format % args)

# ----- SSL Proxy Server Runner -----
def run_proxy(config):
    try:
        listen_port = int(config.get("listen_port", 443))
    except ValueError:
        logger.error("Invalid listen_port in configuration.")
        sys.exit(1)
    server_address = ("", listen_port)
    httpd = http.server.ThreadingHTTPServer(server_address, ProxyHTTPRequestHandler)
    httpd.config = config
    logger.info("Starting SSL proxy on port %s, forwarding to %s:%s",
                listen_port, config["destination_host"], config["destination_port"])

    # Build SSL context from files specified in config.
    ssl_key = config.get("ssl_key")
    ssl_cert = config.get("ssl_cert")
    ssl_ca = config.get("ssl_ca")
    if not (ssl_key and ssl_cert and ssl_ca):
        logger.error("SSL configuration incomplete. Check ssl_key, ssl_cert, and ssl_ca settings.")
        sys.exit(1)
    try:
        context = ssl.create_default_context(ssl.Purpose.CLIENT_AUTH)
        context.load_cert_chain(certfile=ssl_cert, keyfile=ssl_key)
        context.load_verify_locations(cafile=ssl_ca)
        # Optionally require client certs if desired (not enforced here)
        httpd.socket = context.wrap_socket(httpd.socket, server_side=True)
    except Exception as e:
        logger.error("Error setting up SSL: %s", e)
        sys.exit(1)

    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        logger.info("Proxy server shutting down...")
    httpd.server_close()

# ----- Settings Loader -----
def load_settings(settings_file="proxy.ini"):
    config_parser = configparser.ConfigParser()
    if not os.path.exists(settings_file):
        with open(settings_file, "w") as f:
            f.write(DEFAULT_INI)
        logger.info("Created default %s", settings_file)
    config_parser.read(settings_file)
    return dict(config_parser["Proxy"])

# ----- Command-line Parsing -----
def parse_args():
    parser = argparse.ArgumentParser(description="SSL Proxy for R5 Node RPC")
    parser.add_argument("--gencert", action="store_true",
                        help="Generate self-signed certificate, key, and CA bundle in /cert and exit.")
    return parser.parse_args()

# ----- Main Entry Point -----
def main():
    args = parse_args()
    settings = load_settings()

    # If --gencert is provided, generate certificates using cryptography.
    if args.gencert:
        logger.info("Generating self-signed certificate, key, and CA bundle...")
        generate_certificates("cert")
        sys.exit(0)

    # Run the proxy
    run_proxy(settings)

if __name__ == "__main__":
    main()
