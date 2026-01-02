#!/bin/bash

# Exit immediately if a command exits with a non-zero status.
set -e

echo "Pulling latest Docker images for production..."
docker compose -f docker-compose.yml -f docker-compose.prod.yml pull

echo "Starting Docker services in production mode..."
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d

echo "Production services started."
