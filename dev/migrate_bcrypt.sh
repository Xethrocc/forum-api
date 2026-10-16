#!/bin/bash

# Development utility to reset DB with bcrypt hashes for all existing users
# Only run during development - destroys all data

set -e

echo "=== Running Bcrypt Migration ==="

# Reset the database
bash reset_db.sh

echo "=== Migration Complete ==="
echo "All users can now log in with bcrypt-verified passwords."
