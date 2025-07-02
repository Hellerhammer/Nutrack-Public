#!/bin/bash

echo "Generating Swagger documentation..."
cd src/backend/api
swag init -g router.go -o docs --parseDependency
echo "Swagger documentation generated successfully"
