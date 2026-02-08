#!/bin/bash
# Environment variables for kimi-go LLM API
# Usage: source scripts/env.sh

export OPENAI_BASE_URL="https://your-api-endpoint.com/v1"
export OPENAI_API_KEY="your-api-key-here"
export OPENAI_MODEL="your-model-name"

echo "Environment variables set:"
echo "  OPENAI_BASE_URL=$OPENAI_BASE_URL"
echo "  OPENAI_API_KEY=***${OPENAI_API_KEY: -4}"
echo "  OPENAI_MODEL=$OPENAI_MODEL"
