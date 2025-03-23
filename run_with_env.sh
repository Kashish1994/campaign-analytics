#!/bin/bash

# This script runs the API with explicit environment variables to bypass SSL issues

# Set environment variables
export CA_POSTGRES_HOST=localhost
export CA_POSTGRES_PORT=5432
export CA_POSTGRES_DATABASE=campaign_analytics
export CA_POSTGRES_USERNAME=postgres
export CA_POSTGRES_PASSWORD=postgres
export CA_POSTGRES_SSLMODE=disable

# Execute the API with these environment variables
go run cmd/api/main.go

