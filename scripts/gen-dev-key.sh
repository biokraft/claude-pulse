#!/bin/sh
set -e
cd "$(dirname "$0")/.."
[ -f developer_key.der ] && { echo "developer_key.der exists"; exit 0; }
openssl genrsa -out developer_key.pem 4096
openssl pkcs8 -topk8 -inform PEM -outform DER -in developer_key.pem -out developer_key.der -nocrypt
rm developer_key.pem
echo "wrote developer_key.der (gitignored — back it up; store uploads must reuse it)"
