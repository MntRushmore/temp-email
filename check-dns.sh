#!/bin/bash

# DNS Configuration Checker
# Verifies that DNS records are properly configured

set -e

echo "=========================================="
echo "  DNS Configuration Checker"
echo "=========================================="
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Load environment variables
if [ ! -f .env ]; then
    echo -e "${RED}Error: .env file not found!${NC}"
    echo "Run ./setup.sh first to configure the service."
    exit 1
fi

source .env

if [ -z "$DOMAIN" ]; then
    echo -e "${RED}Error: DOMAIN not set in .env${NC}"
    exit 1
fi

echo "Checking DNS records for: $DOMAIN"
echo ""

# Check MX record
echo "=== Checking MX Record ==="
if mx_records=$(dig +short MX "$DOMAIN" 2>/dev/null); then
    if [ -n "$mx_records" ]; then
        echo -e "${GREEN}✓ MX Record found:${NC}"
        echo "$mx_records" | sed 's/^/  /'
    else
        echo -e "${RED}✗ No MX records found for $DOMAIN${NC}"
        echo "  Add: MX record pointing to mail.$DOMAIN"
    fi
else
    echo -e "${RED}✗ Failed to query MX records${NC}"
fi
echo ""

# Check A record for mail subdomain
echo "=== Checking A Record for mail.$DOMAIN ==="
if a_record=$(dig +short A "mail.$DOMAIN" 2>/dev/null); then
    if [ -n "$a_record" ]; then
        echo -e "${GREEN}✓ A Record found:${NC}"
        echo "  $a_record"
    else
        echo -e "${RED}✗ No A record found for mail.$DOMAIN${NC}"
        echo "  Add: A record for 'mail' subdomain"
    fi
else
    echo -e "${RED}✗ Failed to query A records${NC}"
fi
echo ""

# Check web domain
if [ -n "$APP_DOMAIN" ]; then
    web_domain=$(echo "$APP_DOMAIN" | sed 's|https\?://||' | sed 's|/.*||')
    echo "=== Checking A Record for $web_domain ==="
    if web_a_record=$(dig +short A "$web_domain" 2>/dev/null); then
        if [ -n "$web_a_record" ]; then
            echo -e "${GREEN}✓ A Record found:${NC}"
            echo "  $web_a_record"
        else
            echo -e "${RED}✗ No A record found for $web_domain${NC}"
            echo "  Add: A record for web interface domain"
        fi
    else
        echo -e "${RED}✗ Failed to query A records${NC}"
    fi
    echo ""
fi

# Check SPF record
echo "=== Checking SPF Record (Optional) ==="
if spf_record=$(dig +short TXT "$DOMAIN" 2>/dev/null | grep -i "v=spf1"); then
    echo -e "${GREEN}✓ SPF Record found:${NC}"
    echo "  $spf_record"
else
    echo -e "${YELLOW}⚠ No SPF record found (optional but recommended)${NC}"
    echo "  Add: TXT record with value 'v=spf1 mx ~all'"
fi
echo ""

# Summary
echo "=========================================="
echo "  Summary"
echo "=========================================="
echo ""
echo "Domain: $DOMAIN"
echo "Web Interface: ${APP_DOMAIN:-Not configured}"
echo ""
echo "Note: DNS changes can take 5-60 minutes to propagate."
echo "If records are missing, configure them in your DNS provider"
echo "and run this script again to verify."
echo ""
