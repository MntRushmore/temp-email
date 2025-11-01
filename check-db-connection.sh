#!/bin/bash

# Database Connection Diagnostic Tool
# Checks if the database is accessible and working

echo "╔════════════════════════════════════════════════════════════════════╗"
echo "║           DATABASE CONNECTION DIAGNOSTIC TOOL                      ║"
echo "╚════════════════════════════════════════════════════════════════════╝"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "1. Docker Container Status"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
docker-compose ps

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "2. Database Connection Test"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if docker-compose exec -T db psql -U postgres -d temp_email -c "SELECT 1 as connection_ok;" > /dev/null 2>&1; then
    echo "✅ Database connection: OK"
else
    echo "❌ Database connection: FAILED"
    echo "   Check if database container is running"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "3. Database Tables Check"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
docker-compose exec -T db psql -U postgres -d temp_email << EOF
SELECT 
    table_name,
    (SELECT COUNT(*) FROM addresses) as addresses_count,
    (SELECT COUNT(*) FROM emails) as emails_count
FROM information_schema.tables 
WHERE table_schema = 'public' 
AND table_name IN ('addresses', 'emails');
EOF

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "4. Recent Addresses (Last 5)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
docker-compose exec -T db psql -U postgres -d temp_email << EOF
SELECT 
    id,
    created_at,
    expires_at > NOW() as active,
    "user",
    EXTRACT(EPOCH FROM (expires_at - NOW()))/3600 as hours_remaining
FROM addresses 
ORDER BY created_at DESC 
LIMIT 5;
EOF

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "5. Web Server Logs (Last 20 lines)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
docker-compose logs web --tail=20

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "6. Database Errors (if any)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
docker-compose logs db --tail=20 | grep -i error || echo "✅ No errors found in database logs"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "7. Application Errors (ERROR/REJECT logs)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
docker-compose logs web --tail=100 | grep -E "ERROR:|REJECT:" || echo "✅ No ERROR/REJECT logs found"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "8. Success Logs (SUCCESS: Created)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
docker-compose logs web --tail=100 | grep "SUCCESS: Created" | tail -5 || echo "⚠️  No SUCCESS logs found"

echo ""
echo "╔════════════════════════════════════════════════════════════════════╗"
echo "║                    DIAGNOSTICS COMPLETE                            ║"
echo "╚════════════════════════════════════════════════════════════════════╝"
echo ""
echo "Common Issues:"
echo "  ❌ DB connection failed → Database container not running or misconfigured"
echo "  ❌ No tables found → Database not initialized (run migrations)"
echo "  ❌ ERROR logs present → Database insert failing (check error message)"
echo "  ⚠️  No SUCCESS logs → Addresses are failing to create"
echo "  ✅ All checks pass → System is working correctly"
echo ""
echo "Next Steps:"
echo "  1. If database connection failed: docker-compose restart db"
echo "  2. If ERROR logs found: Read the error message for root cause"
echo "  3. If no SUCCESS logs: The bug fix needs to be deployed"
echo "  4. Try creating a new address and re-run this script"
echo ""
