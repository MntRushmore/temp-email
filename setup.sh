#!/bin/bash

# Temp Email Service - Interactive Setup Script
# This script helps you configure the service step by step

set -e

echo "=========================================="
echo "  Temp Email Service - Setup Wizard"
echo "=========================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if .env exists
if [ -f .env ]; then
    echo -e "${YELLOW}Warning: .env file already exists!${NC}"
    read -p "Do you want to overwrite it? (y/N): " overwrite
    if [[ ! $overwrite =~ ^[Yy]$ ]]; then
        echo "Using existing .env file. You can manually edit it if needed."
        exit 0
    fi
fi

echo "Let's configure your Temp Email service!"
echo ""

# Function to prompt for input with default
prompt_input() {
    local prompt="$1"
    local var_name="$2"
    local default="$3"
    local secret="$4"
    
    if [ -n "$default" ]; then
        prompt="$prompt [$default]"
    fi
    
    if [ "$secret" = "secret" ]; then
        read -sp "$prompt: " value
        echo ""
    else
        read -p "$prompt: " value
    fi
    
    if [ -z "$value" ] && [ -n "$default" ]; then
        value="$default"
    fi
    
    eval "$var_name='$value'"
}

echo "=== Slack Configuration ==="
echo "You need to create a Slack App first. See SETUP.md for instructions."
echo ""

prompt_input "Slack Bot Token (starts with xoxb-)" SLACK_TOKEN "" "secret"
prompt_input "Slack Signing Secret" SLACK_SIGNING_SECRET "" "secret"
prompt_input "Slack Channel ID (e.g., C01234567)" SLACK_CHANNEL ""

echo ""
echo "=== Domain Configuration ==="
echo "Make sure you've configured DNS records (see SETUP.md)."
echo ""

prompt_input "Your domain for emails (e.g., tempmail.example.com)" DOMAIN ""
prompt_input "Web interface URL (e.g., https://temp.example.com)" APP_DOMAIN ""

echo ""
echo "=== Database Configuration ==="
echo "Using PostgreSQL with Docker Compose (default settings)."
echo ""

DATABASE_URL="postgres://postgres:postgres@db:5432/temp_email"

# Create .env file
cat > .env << EOF
# Slack Configuration
SLACK_TOKEN=$SLACK_TOKEN
SLACK_SIGNING_SECRET=$SLACK_SIGNING_SECRET
SLACK_CHANNEL=$SLACK_CHANNEL

# Domain Configuration
DOMAIN=$DOMAIN
APP_DOMAIN=$APP_DOMAIN

# Database Configuration
DATABASE_URL=$DATABASE_URL
EOF

echo -e "${GREEN}✓ Configuration saved to .env${NC}"
echo ""

# Validate configuration
echo "=== Validating Configuration ==="

errors=0

if [[ ! $SLACK_TOKEN =~ ^xoxb- ]]; then
    echo -e "${RED}✗ SLACK_TOKEN should start with 'xoxb-'${NC}"
    errors=$((errors+1))
fi

if [ -z "$SLACK_SIGNING_SECRET" ]; then
    echo -e "${RED}✗ SLACK_SIGNING_SECRET is required${NC}"
    errors=$((errors+1))
fi

if [ -z "$SLACK_CHANNEL" ]; then
    echo -e "${RED}✗ SLACK_CHANNEL is required${NC}"
    errors=$((errors+1))
fi

if [ -z "$DOMAIN" ]; then
    echo -e "${RED}✗ DOMAIN is required${NC}"
    errors=$((errors+1))
fi

if [ -z "$APP_DOMAIN" ]; then
    echo -e "${RED}✗ APP_DOMAIN is required${NC}"
    errors=$((errors+1))
fi

if [ $errors -eq 0 ]; then
    echo -e "${GREEN}✓ All configuration values look good!${NC}"
else
    echo -e "${RED}✗ Found $errors error(s). Please check your configuration.${NC}"
    exit 1
fi

echo ""
echo "=== DNS Configuration Checklist ==="
echo "Make sure you've configured these DNS records:"
echo ""
echo "1. MX Record:"
echo "   Type: MX"
echo "   Name: @"
echo "   Priority: 10"
echo "   Value: mail.$DOMAIN"
echo ""
echo "2. A Record (for mail):"
echo "   Type: A"
echo "   Name: mail"
echo "   Value: YOUR_SERVER_IP"
echo ""
echo "3. A Record (for web interface):"
echo "   Type: A"
echo "   Name: temp (or subdomain in APP_DOMAIN)"
echo "   Value: YOUR_SERVER_IP"
echo ""

read -p "Have you configured the DNS records? (y/N): " dns_ready

if [[ ! $dns_ready =~ ^[Yy]$ ]]; then
    echo ""
    echo -e "${YELLOW}Please configure DNS records before starting the service.${NC}"
    echo "See SETUP.md for detailed instructions."
    exit 0
fi

echo ""
echo "=== Ready to Start! ==="
echo ""
read -p "Do you want to start the service now? (y/N): " start_now

if [[ $start_now =~ ^[Yy]$ ]]; then
    echo ""
    echo "Starting services with Docker Compose..."
    docker-compose up -d
    echo ""
    echo -e "${GREEN}✓ Services started!${NC}"
    echo ""
    echo "View logs with: docker-compose logs -f"
    echo ""
    echo "=== Next Steps ==="
    echo "1. Wait for services to fully start (check logs)"
    echo "2. Update Slack Event Subscriptions URL to: $APP_DOMAIN/slack/events"
    echo "3. Test by posting 'gib email' in your Slack channel"
    echo ""
else
    echo ""
    echo "Configuration complete! Start the service when ready with:"
    echo "  docker-compose up -d"
    echo ""
fi

echo "=== Quick Reference ==="
echo "• View logs: docker-compose logs -f"
echo "• Restart: docker-compose restart"
echo "• Stop: docker-compose down"
echo "• See full documentation: cat SETUP.md"
echo ""
echo -e "${GREEN}Setup complete! 🎉${NC}"
