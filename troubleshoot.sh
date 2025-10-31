#!/bin/bash

# Troubleshooting Script
# Diagnoses common issues with the Temp Email service

set -e

echo "=========================================="
echo "  Temp Email - Troubleshooting"
echo "=========================================="
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Check if .env exists
if [ ! -f .env ]; then
    echo -e "${RED}✗ .env file not found!${NC}"
    echo "  Run ./setup.sh to configure the service."
    exit 1
fi

echo -e "${BLUE}[1/7] Checking configuration...${NC}"
source .env

config_ok=true
if [ -z "$SLACK_TOKEN" ]; then
    echo -e "${RED}✗ SLACK_TOKEN not set${NC}"
    config_ok=false
fi

if [ -z "$SLACK_SIGNING_SECRET" ]; then
    echo -e "${RED}✗ SLACK_SIGNING_SECRET not set${NC}"
    config_ok=false
fi

if [ -z "$SLACK_CHANNEL" ]; then
    echo -e "${RED}✗ SLACK_CHANNEL not set${NC}"
    config_ok=false
fi

if [ -z "$DOMAIN" ]; then
    echo -e "${RED}✗ DOMAIN not set${NC}"
    config_ok=false
fi

if $config_ok; then
    echo -e "${GREEN}✓ Configuration looks good${NC}"
else
    echo -e "${RED}✗ Configuration has errors${NC}"
    echo "  Edit .env file to fix the issues"
fi
echo ""

echo -e "${BLUE}[2/7] Checking Docker...${NC}"
if command -v docker &> /dev/null; then
    echo -e "${GREEN}✓ Docker is installed${NC}"
    docker --version
    
    if command -v docker-compose &> /dev/null; then
        echo -e "${GREEN}✓ Docker Compose is installed${NC}"
        docker-compose --version
    else
        echo -e "${RED}✗ Docker Compose not found${NC}"
        echo "  Install docker-compose to run the service"
    fi
else
    echo -e "${RED}✗ Docker is not installed${NC}"
    echo "  Install Docker to run the service"
fi
echo ""

echo -e "${BLUE}[3/7] Checking service status...${NC}"
if docker-compose ps 2>/dev/null | grep -q "Up"; then
    echo -e "${GREEN}✓ Services are running${NC}"
    docker-compose ps
    
    # Check if main service is healthy
    if docker-compose ps | grep "main" | grep -q "Up"; then
        echo -e "${GREEN}✓ Main service is up${NC}"
    else
        echo -e "${RED}✗ Main service is not running${NC}"
        echo "  Run: docker-compose logs main"
    fi
    
    # Check if database is healthy
    if docker-compose ps | grep "db" | grep -q "Up"; then
        echo -e "${GREEN}✓ Database is up${NC}"
    else
        echo -e "${RED}✗ Database is not running${NC}"
        echo "  Run: docker-compose logs db"
    fi
else
    echo -e "${YELLOW}⚠ Services are not running${NC}"
    echo "  Start with: docker-compose up -d"
fi
echo ""

echo -e "${BLUE}[4/7] Checking DNS configuration...${NC}"
if [ -n "$DOMAIN" ]; then
    mx_records=$(dig +short MX "$DOMAIN" 2>/dev/null || echo "")
    if [ -n "$mx_records" ]; then
        echo -e "${GREEN}✓ MX record found${NC}"
    else
        echo -e "${RED}✗ No MX record found${NC}"
        echo "  Add MX record in your DNS settings"
    fi
    
    a_record=$(dig +short A "mail.$DOMAIN" 2>/dev/null || echo "")
    if [ -n "$a_record" ]; then
        echo -e "${GREEN}✓ A record for mail.$DOMAIN found${NC}"
    else
        echo -e "${RED}✗ No A record for mail.$DOMAIN${NC}"
        echo "  Add A record for 'mail' subdomain"
    fi
else
    echo -e "${YELLOW}⚠ DOMAIN not configured${NC}"
fi
echo ""

echo -e "${BLUE}[5/7] Checking ports...${NC}"
ports_ok=true

# Check port 25 (SMTP)
if netstat -tuln 2>/dev/null | grep -q ":25 " || ss -tuln 2>/dev/null | grep -q ":25 "; then
    echo -e "${GREEN}✓ Port 25 (SMTP) is listening${NC}"
else
    echo -e "${YELLOW}⚠ Port 25 (SMTP) not listening${NC}"
    echo "  Make sure Docker is running and port mapping is correct"
    ports_ok=false
fi

# Check port 3001 (HTTP)
if netstat -tuln 2>/dev/null | grep -q ":3001 " || ss -tuln 2>/dev/null | grep -q ":3001 "; then
    echo -e "${GREEN}✓ Port 3001 (HTTP) is listening${NC}"
else
    echo -e "${YELLOW}⚠ Port 3001 (HTTP) not listening${NC}"
    echo "  Make sure Docker is running and port mapping is correct"
    ports_ok=false
fi

if ! $ports_ok; then
    echo "  Run: docker-compose ps"
fi
echo ""

echo -e "${BLUE}[6/7] Checking recent logs...${NC}"
if docker-compose ps 2>/dev/null | grep -q "Up"; then
    echo "Last 10 lines from main service:"
    echo "---"
    docker-compose logs --tail=10 main 2>/dev/null || echo "Could not retrieve logs"
    echo "---"
else
    echo -e "${YELLOW}⚠ Services not running, cannot check logs${NC}"
fi
echo ""

echo -e "${BLUE}[7/7] Checking connectivity...${NC}"

# Check if we can reach Slack API
if command -v curl &> /dev/null; then
    if [ -n "$SLACK_TOKEN" ]; then
        echo "Testing Slack API connection..."
        response=$(curl -s -H "Authorization: Bearer $SLACK_TOKEN" https://slack.com/api/auth.test)
        if echo "$response" | grep -q '"ok":true'; then
            echo -e "${GREEN}✓ Slack API connection successful${NC}"
        else
            echo -e "${RED}✗ Slack API connection failed${NC}"
            echo "  Check your SLACK_TOKEN"
            echo "  Response: $response"
        fi
    fi
else
    echo -e "${YELLOW}⚠ curl not installed, skipping connectivity tests${NC}"
fi
echo ""

echo "=========================================="
echo "  Troubleshooting Summary"
echo "=========================================="
echo ""
echo "Common issues and solutions:"
echo ""
echo "1. Services won't start:"
echo "   • Check: docker-compose logs"
echo "   • Try: docker-compose down && docker-compose up -d"
echo ""
echo "2. Not receiving emails:"
echo "   • Check DNS records: ./check-dns.sh"
echo "   • Verify port 25 is open"
echo "   • Check logs: docker-compose logs main"
echo ""
echo "3. Slack events not working:"
echo "   • Verify SLACK_SIGNING_SECRET is correct"
echo "   • Update Event Subscriptions URL in Slack app settings"
echo "   • Check port 3001 is accessible from internet"
echo ""
echo "4. Bot not responding:"
echo "   • Check SLACK_TOKEN has correct permissions"
echo "   • Verify bot is invited to the channel"
echo "   • Confirm SLACK_CHANNEL ID is correct"
echo ""
echo "For detailed logs: docker-compose logs -f"
echo "For DNS checking: ./check-dns.sh"
echo ""
