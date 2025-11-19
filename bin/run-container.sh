#!/bin/sh

# Container entry point script

if [ -z "$AWS_SECRET_ARN_RDS_USER" ]; then
  echo "Error: AWS_SECRET_ARN_RDS_USER is not set"
  exit 1
fi

exec /app/scapi --log stdout --config sm+json://$AWS_SECRET_ARN_RDS_USER