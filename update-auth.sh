#!/bin/bash

# Update Auth - Simple deployment script for password authentication
# Usage: ./update-auth.sh [password]

set -e

echo "🔐 Updating to Simple Password Authentication..."
echo ""

# Pull latest code
echo "📥 Pulling latest code from GitHub..."
git pull origin master

# Check if password was provided as argument
if [ -n "$1" ]; then
    PASSWORD="$1"
else
    # Prompt for password
    echo ""
    read -p "Enter dashboard password: " PASSWORD
fi

# Check if .env exists
if [ ! -f .env ]; then
    echo "❌ Error: .env file not found!"
    echo "Please create .env file first (see .env.example)"
    exit 1
fi

# Check if DASHBOARD_PASSWORD already exists in .env
if grep -q "^DASHBOARD_PASSWORD=" .env; then
    echo "✏️  Updating DASHBOARD_PASSWORD in .env..."
    sed -i "s/^DASHBOARD_PASSWORD=.*/DASHBOARD_PASSWORD=$PASSWORD/" .env
else
    echo "➕ Adding DASHBOARD_PASSWORD to .env..."
    echo "" >> .env
    echo "# Dashboard Authentication" >> .env
    echo "DASHBOARD_PASSWORD=$PASSWORD" >> .env
fi

# Remove old OAuth variables if they exist
if grep -q "^LOGIN_CLIENT_ID=" .env || grep -q "^LOGIN_CLIENT_SECRET=" .env; then
    echo "🧹 Removing old OAuth variables..."
    sed -i '/^LOGIN_CLIENT_ID=/d' .env
    sed -i '/^LOGIN_CLIENT_SECRET=/d' .env
    sed -i '/^SKIP_AUTH=/d' .env
fi

echo ""
echo "📦 Rebuilding Docker containers..."
docker-compose down
docker-compose up -d --build

echo ""
echo "✅ Authentication updated successfully!"
echo ""
echo "🌐 Dashboard: https://mail.rushilchopra.com/dashboard"
echo "🔑 Password: $PASSWORD"
echo ""
echo "⏳ Waiting for services to start..."
sleep 5

echo ""
echo "📊 Service Status:"
docker-compose ps

echo ""
echo "✨ Done! Try accessing your dashboard now."
echo ""
echo "💡 Tips:"
echo "  - All existing sessions are invalidated"
echo "  - Users will need to login with the new password"
echo "  - Password is stored in .env file"
echo "  - Change password by running this script again"
echo ""
