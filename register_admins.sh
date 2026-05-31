#!/bin/bash
BASE_URL="http://localhost:8000/v1/users/register"

for i in 1 2 3 4; do
    echo -n "admin$i: "
    curl -s -X POST "$BASE_URL" \
        -H "Content-Type: application/json" \
        -d "{\"name\":\"admin$i\",\"password\":\"123456\"}"
    echo
done
