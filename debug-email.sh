#!/bin/bash

# Debug Email Address Issue
# Usage: ./debug-email.sh [address_id]

ADDRESS_ID=${1:-ntujye}

echo "╔═══════════════════════════════════════════════════════════════════╗"
echo "║                EMAIL ADDRESS DEBUG TOOL                           ║"
echo "╚═══════════════════════════════════════════════════════════════════╝"
echo ""
echo "Checking address: ${ADDRESS_ID}@rushilchopra.com"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "1. Database Check"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
docker-compose exec -T db psql -U postgres -d temp_email << EOF
SELECT 
    id,
    created_at,
    expires_at,
    expires_at > NOW() as is_active,
    EXTRACT(EPOCH FROM (expires_at - NOW()))/3600 as hours_remaining,
    "user",
    timestamp
FROM addresses 
WHERE id = '${ADDRESS_ID}';
EOF

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "2. All Addresses (Last 10)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
docker-compose exec -T db psql -U postgres -d temp_email << EOF
SELECT 
    id,
    created_at,
    expires_at > NOW() as active,
    "user"
FROM addresses 
ORDER BY created_at DESC 
LIMIT 10;
EOF

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "3. Service Status"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
docker-compose ps

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "4. Recent SMTP Logs"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
docker-compose logs smtp --tail=30

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "5. Recent Web Logs"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
docker-compose logs web --tail=20

echo ""
echo "╔═══════════════════════════════════════════════════════════════════╗"
echo "║                         DIAGNOSTICS COMPLETE                      ║"
echo "╚═══════════════════════════════════════════════════════════════════╝"
echo ""
echo "Common Issues:"
echo "  • Address expired (check expires_at vs NOW())"
echo "  • Address never created (check if id exists)"
echo "  • Database connection issue (check service status)"
echo "  • SMTP server not running (check logs)"
echo ""
