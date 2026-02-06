#!/bin/bash

# Simple local install script for forklift

set -e

echo "Building forklift..."
go build -o forklift .

echo "Installing forklift to /usr/local/bin..."
if [ -w "/usr/local/bin" ]; then
    mv forklift /usr/local/bin/
else
    sudo mv forklift /usr/local/bin/
fi

echo "Success! You can now run 'forklift init'"
