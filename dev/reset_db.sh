#!/bin/bash

set -e

echo "=== Resetting Forum Database ==="

# Drop database if exists
psql -h localhost -U postgres -c "DROP DATABASE IF EXISTS forum;"

# Create database
psql -h localhost -U postgres -c "CREATE DATABASE forum;"

# Load schema
psql -h localhost -U postgres -d forum -f ../docs/schema.sql

echo "=== Database Reset Complete ==="
