#!/bin/bash

BASE_URL="http://localhost:8080"

echo "=== Forum API Test Suite ==="

echo -e "\n1. Create User (Register)"
curl -s -X POST "$BASE_URL/api/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","email":"alice@example.com","password":"test123"}'

echo -e "\n\n2. Login"
TOKEN_RESPONSE=$(curl -s -X POST "$BASE_URL/api/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","password":"test123"}')
echo "$TOKEN_RESPONSE"

echo -e "\n3. Create Forum"
curl -s -X POST "$BASE_URL/api/forums" \
  -H "Content-Type: application/json" \
  -d '{"title":"General Discussion","description":"Talk about anything"}'

echo -e "\n4. List Forums"
curl -s "$BASE_URL/api/forums"

echo -e "\n5. Create Thread"
curl -s -X POST "$BASE_URL/api/forums/1/threads" \
  -H "Content-Type: application/json" \
  -d '{"title":"Welcome Thread","author_id":1}'

echo -e "\n6. Get Thread with Posts"
curl -s "$BASE_URL/api/threads/1"

echo -e "\n7. Post Reply"
curl -s -X POST "$BASE_URL/api/threads/1/posts" \
  -H "Content-Type: application/json" \
  -d '{"content":"Hello everyone!","author_id":1}'

echo -e "\n8. Send Message"
curl -s -X POST "$BASE_URL/api/messages" \
  -H "Content-Type: application/json" \
  -d '{"recipient_id":2,"content":"Hey there!"}'

echo -e "\n9. Get Messages"
curl -s "$BASE_URL/api/messages"

echo -e "\n10. Search"
curl -s "$BASE_URL/api/search?q=hello"

echo -e "\n11. Landing Page"
curl -s "$BASE_URL/"

echo -e "\n\n=== Tests Complete ==="
