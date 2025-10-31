#!/bin/bash

# Test Address Creation Fix
# This script helps verify the 550 Recipient Rejected bug fix is working

set -e

echo "╔════════════════════════════════════════════════════════════════════╗"
echo "║         ADDRESS CREATION BUG FIX VERIFICATION TOOL                 ║"
echo "╚════════════════════════════════════════════════════════════════════╝"
echo ""

# Get address ID from argument or prompt
if [ -z "$1" ]; then
    echo "Usage: $0 <address-id>"
    echo ""
    echo "Example: $0 abc123"
    echo ""
    echo "This will:"
    echo "  1. Check if the address exists in the database"
    echo "  2. Show creation logs for that address"
    echo "  3. Show recent error logs"
    echo "  4. Show recent success logs"
    echo ""
    read -p "Enter address ID to check: " ADDRESS_ID
else
    ADDRESS_ID=$1
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "1. Checking if address exists: ${ADDRESS_ID}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Try to use curl to check the debug endpoint
if command -v curl &> /dev/null; then
    echo "Using debug endpoint (GET /api/check/${ADDRESS_ID})..."
    RESPONSE=$(curl -s "http://localhost:3001/api/check/${ADDRESS_ID}" || echo '{"error": "connection failed"}')
    echo "$RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$RESPONSE"
else
    echo "curl not available, checking database directly..."
    docker-compose exec -T db psql -U postgres -d temp_email << EOF
SELECT 
    id,
    created_at,
    expires_at,
    expires_at > NOW() as is_active,
    EXTRACT(EPOCH FROM (expires_at - NOW()))/3600 as hours_remaining,
    "user"
FROM addresses 
WHERE id = '${ADDRESS_ID}';
EOF
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "2. Recent SUCCESS logs (last 10)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
docker-compose logs web --tail=1000 | grep "SUCCESS: Created" | tail -10 || echo "No SUCCESS logs found"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "3. Recent ERROR logs (last 10)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
docker-compose logs web --tail=1000 | grep "ERROR: Failed to create" | tail -10 || echo "✅ No ERROR logs found (good!)"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "4. Recent SMTP logs for this address"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
docker-compose logs smtp --tail=1000 | grep -i "${ADDRESS_ID}" | tail -10 || echo "No SMTP logs found for this address"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "5. Recent ACCEPT/REJECT logs (last 10)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
docker-compose logs smtp --tail=1000 | grep -E "ACCEPT:|REJECT:" | tail -10 || echo "No ACCEPT/REJECT logs found"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "6. All addresses created in last 24 hours"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
docker-compose exec -T db psql -U postgres -d temp_email << EOF
SELECT 
    id,
    created_at,
    expires_at > NOW() as active,
    "user",
    EXTRACT(EPOCH FROM (NOW() - created_at))/3600 as hours_ago
FROM addresses 
WHERE created_at > NOW() - INTERVAL '24 hours'
ORDER BY created_at DESC;
EOF

echo ""
echo "╔════════════════════════════════════════════════════════════════════╗"
echo "║                     VERIFICATION COMPLETE                          ║"
echo "╚════════════════════════════════════════════════════════════════════╝"
echo ""
echo "Interpretation:"
echo "  ✅ Address found + SUCCESS log = Bug is FIXED"
echo "  ❌ Address NOT found + ERROR log = Database issue (check error message)"
echo "  ❌ Address NOT found + NO logs = Address was never created"
echo "  ⚠️  REJECT logs = Address expired or was deleted"
echo ""
echo "Next steps if issues persist:"
echo "  1. Check database connection: docker-compose ps"
echo "  2. Review full logs: docker-compose logs web | less"
echo "  3. Test with new address: have friend type 'gib email' again"
echo "  4. Use debug endpoint: curl http://localhost:3001/api/check/[address-id]"
echo ""
