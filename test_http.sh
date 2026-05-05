#!/bin/bash

# Simple iKuai v4 API test

IKUAI_ADDR="http://10.10.30.254"
IKUAI_USERNAME="zhangyi"
IKUAI_PASSWORD="zx19950124"

echo "=== iKuai OS v4 API Test ==="
echo "Router: $IKUAI_ADDR"
echo ""

# Function to compute MD5
compute_md5() {
    printf "%s" "$1" | md5sum | awk '{print $1}'
}

# Function to compute base64
compute_base64() {
    printf "%s" "salt_11$1" | base64
}

# Generate password hash
PASS_MD5=$(compute_md5 "$IKUAI_PASSWORD")
PASS_BASE64=$(compute_base64 "salt_11$IKUAI_PASSWORD")

echo "Password MD5: $PASS_MD5"
echo "Password Base64: $PASS_BASE64"
echo ""

# Test 1: Login
echo "--- Test 1: Login ---"
LOGIN_RESPONSE=$(curl -s -X POST "$IKUAI_ADDR/Action/login" \
    -H "Content-Type: application/json" \
    -d '{
        "username": "'$IKUAI_USERNAME'",
        "passwd": "'$PASS_MD5'",
        "pass": "'$PASS_BASE64'"
    }')

echo "Login Response:"
echo "$LOGIN_RESPONSE"
echo ""

# Check response
RESULT=$(echo "$LOGIN_RESPONSE" | grep -o '"Result":[0-9]*')
CODE=$(echo "$LOGIN_RESPONSE" | grep -o '"code":[0-9]*')

if [ -n "$RESULT" ]; then
    echo "v3 format: Result=$RESULT"
fi

if [ -n "$CODE" ]; then
    echo "v4 format: code=$CODE"
fi

echo ""
