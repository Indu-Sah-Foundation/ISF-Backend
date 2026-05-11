#!/bin/bash
# Save as test-api.sh, then: chmod +x test-api.sh && ./test-api.sh

BASE="https://isfinfa-go-backend.azurewebsites.net"
ADMIN_EMAIL="admin@isf.org"
ADMIN_PASS="ChangeMe-RotateImmediately"   # whatever you set

# Color helpers
GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'; NC='\033[0m'
pass() { echo -e "${GREEN}✓${NC} $1"; }
fail() { echo -e "${RED}✗${NC} $1"; }
info() { echo -e "${YELLOW}→${NC} $1"; }

# Helper: assert HTTP status
expect() {
  local method=$1 url=$2 expected=$3 description=$4 data=$5 token=$6
  local args=(-s -o /tmp/body.txt -w "%{http_code}" -X "$method" "$url")
  [[ -n "$data"  ]] && args+=(-H 'Content-Type: application/json' -d "$data")
  [[ -n "$token" ]] && args+=(-H "Authorization: Bearer $token")
  local code=$(curl "${args[@]}")
  if [[ "$code" == "$expected" ]]; then
    pass "$description (got $code)"
  else
    fail "$description: expected $expected, got $code"
    cat /tmp/body.txt; echo
  fi
}

echo "=== HEALTH ==="
expect GET "$BASE/health"     200 "GET /health returns 200"
expect GET "$BASE/health/db"  200 "GET /health/db returns 200"

echo
echo "=== AUTH ==="
expect POST "$BASE/auth/login" 400 "POST /auth/login with no body returns 400"
expect POST "$BASE/auth/login" 401 "POST /auth/login with wrong password returns 401" \
  '{"email":"admin@isf.org","password":"definitely-wrong-pass-12345"}'
expect POST "$BASE/auth/login" 200 "POST /auth/login with correct creds returns 200" \
  "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASS\"}"

# Capture a token for the protected requests
TOKEN=$(curl -s -X POST "$BASE/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASS\"}" | jq -r .token)
[[ -z "$TOKEN" || "$TOKEN" == "null" ]] && { fail "could not get token, aborting"; exit 1; }
info "got JWT (${TOKEN:0:30}...)"

echo
echo "=== AUTHORIZATION ENFORCEMENT ==="
expect POST "$BASE/people" 401 "POST /people without token returns 401" \
  '{"name":"x","email":"x@y.com"}'
expect GET "$BASE/people" 401 "GET /people without token returns 401"
expect POST "$BASE/articles" 401 "POST /articles without token returns 401" \
  '{"slug":"x","title":"x","body_md":"x"}'
expect GET "$BASE/people" 401 "GET /people with tampered token returns 401" "" "${TOKEN}xxx"

echo
echo "=== PEOPLE ==="
expect POST "$BASE/people" 400 "POST /people with bad email returns 400" \
  '{"name":"Bad","email":"not-email"}' "$TOKEN"
expect POST "$BASE/people" 201 "POST /people with valid body returns 201" \
  '{"name":"Test User","email":"test+'$RANDOM'@example.com"}' "$TOKEN"
expect GET "$BASE/people" 200 "GET /people with token returns 200" "" "$TOKEN"

PERSON_ID=$(curl -s "$BASE/people" -H "Authorization: Bearer $TOKEN" | jq -r '.[0].id')
expect GET "$BASE/people/$PERSON_ID" 200 "GET /people/:id returns 200" "" "$TOKEN"
expect GET "$BASE/people/00000000-0000-0000-0000-000000000000" 404 "GET /people/<bad> returns 404" "" "$TOKEN"
expect GET "$BASE/people/not-a-uuid" 400 "GET /people/<malformed> returns 400" "" "$TOKEN"

echo
echo "=== ARTICLES (public reads) ==="
expect GET "$BASE/articles" 200 "GET /articles (public) returns 200"
expect GET "$BASE/articles/this-slug-does-not-exist" 404 "GET /articles/<missing> returns 404"

echo
echo "=== ARTICLES (admin writes) ==="
SLUG="test-$RANDOM"
expect POST "$BASE/articles" 201 "POST /articles creates an article" \
  "{\"slug\":\"$SLUG\",\"title\":\"Test Article\",\"body_md\":\"# Hello\\n\\nBody.\",\"publish\":true}" "$TOKEN"

ARTICLE_ID=$(curl -s "$BASE/articles" | jq -r ".[] | select(.slug==\"$SLUG\") | .id")
info "created article id=$ARTICLE_ID"

expect GET "$BASE/articles/$SLUG" 200 "GET /articles/$SLUG (public) returns 200"
expect PUT "$BASE/articles/$ARTICLE_ID" 200 "PUT /articles/:id updates title" \
  '{"title":"Updated Title"}' "$TOKEN"

# Verify the update landed
NEW_TITLE=$(curl -s "$BASE/articles/$SLUG" | jq -r .title)
[[ "$NEW_TITLE" == "Updated Title" ]] && pass "title was actually updated" || fail "title not updated (got: $NEW_TITLE)"

echo
echo "=== TRANSLATION ==="
expect GET "$BASE/translate/languages" 200 "GET /translate/languages (public) returns 200"

# Cold translation -- first request hits Translator API
info "cold ES translation:"
time curl -s "$BASE/articles/$SLUG?lang=es" | jq '{title, body_md}'

# Warm -- should hit Redis
info "warm ES translation (should be faster):"
time curl -s "$BASE/articles/$SLUG?lang=es" | jq .title

echo
echo "=== DONATIONS ==="
expect GET "$BASE/donations/amounts" 200 "GET /donations/amounts (public) returns 200"
expect GET "$BASE/donations" 401 "GET /donations without token returns 401"
expect GET "$BASE/donations" 200 "GET /donations with token returns 200" "" "$TOKEN"

expect POST "$BASE/donations/checkout" 400 "POST /donations/checkout with no amount returns 400" '{}'
expect POST "$BASE/donations/checkout" 400 "POST /donations/checkout with too-low amount returns 400" \
  '{"amount_cents":50}'

echo "→ Real Stripe checkout (will return URL):"
curl -s -X POST "$BASE/donations/checkout" \
  -H 'Content-Type: application/json' \
  -d '{"amount_cents":5000,"email":"donor@example.com"}' | jq

echo
echo "=== CLEANUP ==="
expect DELETE "$BASE/articles/$ARTICLE_ID" 204 "DELETE /articles/:id returns 204" "" "$TOKEN"
expect GET "$BASE/articles/$SLUG" 404 "deleted article is gone (404)"

echo
echo "Done."