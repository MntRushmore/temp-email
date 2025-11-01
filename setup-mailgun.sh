#!/bin/bash

# Mailgun Quick Setup Script

echo "╔════════════════════════════════════════════════════════════════════╗"
echo "║              MAILGUN INTEGRATION SETUP                             ║"
echo "╚════════════════════════════════════════════════════════════════════╝"
echo ""

# Check if .env exists
if [ ! -f .env ]; then
    echo "❌ .env file not found!"
    echo "Creating from .env.example..."
    cp .env.example .env
fi

echo "📝 Please provide your Mailgun details:"
echo ""

# Get API key
read -p "Mailgun API Key: " API_KEY
read -p "Mailgun Domain [sandbox8822e8e06d904455a74c0d9d6375ecd3.mailgun.org]: " DOMAIN
DOMAIN=${DOMAIN:-sandbox8822e8e06d904455a74c0d9d6375ecd3.mailgun.org}

read -p "Mailgun Signing Key (optional, press Enter to skip): " SIGNING_KEY

# Update .env file
echo ""
echo "📝 Updating .env file..."

# Check if Mailgun vars already exist
if grep -q "MAILGUN_API_KEY=" .env; then
    # Update existing
    sed -i "s|MAILGUN_API_KEY=.*|MAILGUN_API_KEY=$API_KEY|" .env
    sed -i "s|MAILGUN_DOMAIN=.*|MAILGUN_DOMAIN=$DOMAIN|" .env
    sed -i "s|MAILGUN_SIGNING_KEY=.*|MAILGUN_SIGNING_KEY=$SIGNING_KEY|" .env
else
    # Add new
    echo "" >> .env
    echo "# Mailgun Configuration" >> .env
    echo "MAILGUN_API_KEY=$API_KEY" >> .env
    echo "MAILGUN_DOMAIN=$DOMAIN" >> .env
    echo "MAILGUN_SIGNING_KEY=$SIGNING_KEY" >> .env
fi

echo "✅ Environment variables updated"
echo ""

# Ask about deployment
read -p "Deploy now? (y/n): " DEPLOY

if [ "$DEPLOY" = "y" ] || [ "$DEPLOY" = "Y" ]; then
    echo ""
    echo "🚀 Deploying..."
    echo ""
    
    docker-compose build --no-cache
    docker-compose down
    docker-compose up -d
    
    echo ""
    echo "✅ Deployment complete!"
    echo ""
    echo "📊 Checking status..."
    docker-compose ps
    echo ""
    
    echo "📝 Recent logs:"
    docker-compose logs main --tail=20
fi

echo ""
echo "╔════════════════════════════════════════════════════════════════════╗"
echo "║                     NEXT STEPS                                     ║"
echo "╚════════════════════════════════════════════════════════════════════╝"
echo ""
echo "1. Configure Mailgun Route:"
echo "   Go to: https://app.mailgun.com/app/sending/domains"
echo "   Click your domain → Routes → Create Route"
echo ""
echo "   Expression: match_recipient('.*@$DOMAIN')"
echo "   Action: forward('https://mail.rushilchopra.com/webhook/mailgun')"
echo ""
echo "2. Get Signing Key (if you skipped it):"
echo "   Go to: https://app.mailgun.com/app/account/security/api_keys"
echo "   Copy 'HTTP webhook signing key'"
echo "   Add to .env: MAILGUN_SIGNING_KEY=your-key"
echo "   Restart: docker-compose restart main"
echo ""
echo "3. Test email delivery:"
echo "   Create address in dashboard: test123"
echo "   Send email to: test123@$DOMAIN"
echo ""
echo "4. Check logs:"
echo "   docker-compose logs main | grep -E 'Mailgun webhook|ACCEPT|REJECT'"
echo ""
echo "📚 Full documentation: See MAILGUN-SETUP.md"
echo ""
