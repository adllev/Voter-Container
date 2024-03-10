#!/bin/bash
docker buildx create --use 
docker buildx build --platform linux/amd64,linux/arm64 -f ./dockerfile.better . -t architectingsoftware/voter-api:v5 --push