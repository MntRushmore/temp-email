# Temp Email Service - Complete Setup Guide

## Prerequisites

Before you begin, you'll need:

1. **A domain name** (e.g., `example.com`)
2. **A Slack workspace** with admin access
3. **A server** with:
   - Docker and Docker Compose installed
   - Ports 25 (SMTP) and 3001 (HTTP) accessible
   - Public IP address

## Step 1: DNS Configuration

Configure your domain's DNS records:

### A. Add MX Record
```
Type: MX
Name: @ (or your subdomain)
Priority: 10
Value: mail.yourdomain.com
```

### B. Add A Record (for mail server)
```
Type: A
Name: mail
Value: YOUR_SERVER_IP_ADDRESS
```

### C. Add A Record (for web interface)
```
Type: A
Name: temp (or any subdomain for viewing emails)
Value: YOUR_SERVER_IP_ADDRESS
```

### D. Add SPF Record (Optional but recommended)
```
Type: TXT
Name: @
Value: v=spf1 mx ~all
```

### E. Test DNS Propagation
Wait 5-15 minutes, then test:
```bash
dig yourdomain.com MX
dig mail.yourdomain.com A
nslookup yourdomain.com
```

## Step 2: Create Slack App

### A. Create New Slack App
1. Go to https://api.slack.com/apps
2. Click **"Create New App"** → **"From scratch"**
3. Name: `Temp Email Bot`
4. Choose your workspace

### B. Configure OAuth & Permissions
1. Navigate to **"OAuth & Permissions"**
2. Add these **Bot Token Scopes**:
   ```
   channels:history
   channels:read
   chat:write
   reactions:write
   ```
3. Click **"Install to Workspace"**
4. **Copy the Bot User OAuth Token** (starts with `xoxb-`)

### C. Enable Event Subscriptions
1. Navigate to **"Event Subscriptions"**
2. Toggle **"Enable Events"** to ON
3. **Request URL**: `https://temp.yourdomain.com/slack/events`
   - Replace `temp.yourdomain.com` with your actual domain
   - You'll need to start the service first for verification
4. Subscribe to **bot events**:
   ```
   message.channels
   ```
5. Click **"Save Changes"**

### D. Get Signing Secret
1. Navigate to **"Basic Information"**
2. Scroll to **"App Credentials"**
3. **Copy the Signing Secret**

### E. Create Slack Channel
1. In your Slack workspace, create a channel (e.g., `#temp-emails`)
2. Invite your bot to the channel: `/invite @Temp Email Bot`
3. **Copy the Channel ID**:
   - Right-click channel → View channel details
   - Scroll down to see the Channel ID

## Step 3: Configure Environment Variables

Create a `.env` file in the project root:

```bash
# Copy the example file
cp .env.example .env
```

Edit `.env` with your values:

```bash
# Slack Configuration
SLACK_TOKEN=xoxb-your-bot-token-here
SLACK_SIGNING_SECRET=your-signing-secret-here
SLACK_CHANNEL=C01234567  # Your channel ID

# Domain Configuration
DOMAIN=yourdomain.com  # The domain for email addresses (e.g., user@yourdomain.com)
APP_DOMAIN=https://temp.yourdomain.com  # The web interface URL

# Database Configuration (automatically set by docker-compose)
DATABASE_URL=postgres://postgres:postgres@db:5432/temp_email
```

### Example Configuration
```bash
SLACK_TOKEN=xoxb-YOUR-BOT-TOKEN-HERE
SLACK_SIGNING_SECRET=your-signing-secret-here
SLACK_CHANNEL=C04EXAMPLE
DOMAIN=tempmail.example.com
APP_DOMAIN=https://temp.tempmail.example.com
```

## Step 4: Server Setup

### A. Ensure Firewall Rules
Make sure your server allows:
- Port **25** (SMTP) - for receiving emails
- Port **3001** (HTTP) - for Slack webhooks and web interface
- Port **80/443** (if using reverse proxy)

```bash
# For UFW (Ubuntu/Debian)
sudo ufw allow 25/tcp
sudo ufw allow 3001/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
```

### B. Optional: Setup Reverse Proxy (Recommended)
Use Nginx or Caddy to handle HTTPS:

**Caddy Example** (`Caddyfile`):
```
temp.yourdomain.com {
    reverse_proxy localhost:3001
}
```

**Nginx Example**:
```nginx
server {
    listen 80;
    server_name temp.yourdomain.com;
    
    location / {
        proxy_pass http://localhost:3001;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## Step 5: Build and Run

### A. Clone and Build
```bash
cd /home/user/webapp
docker-compose up -d
```

### B. Check Logs
```bash
docker-compose logs -f
```

You should see:
```
Starting up SMTP server...
Starting up HTTP server...
```

### C. Verify Slack Event URL
After the service starts, Slack will attempt to verify the Events URL. Check logs for the verification request.

## Step 6: Testing

### A. Test in Slack
1. Go to your Slack channel
2. Post: `gib email`
3. The bot should respond with a temporary email address like:
   ```
   wahoo! your temporary 24-hour email address is abc123@yourdomain.com
   ```

### B. Test Email Reception
Send an email to the generated address:
```bash
echo "Test email body" | mail -s "Test Subject" abc123@yourdomain.com
```

The email should appear as a reply in the Slack thread.

### C. Test Web Viewing
Click the browser link in the Slack message to view the email in your browser.

## Troubleshooting

### Issue: Slack verification fails
- Ensure the service is running and accessible
- Check firewall rules
- Verify the Request URL matches your domain
- Check `docker-compose logs` for errors

### Issue: Not receiving emails
- Verify MX records: `dig yourdomain.com MX`
- Check SMTP server is running on port 25: `docker-compose ps`
- Test SMTP connection: `telnet mail.yourdomain.com 25`
- Check logs: `docker-compose logs main`

### Issue: Database connection errors
- Ensure PostgreSQL container is running: `docker-compose ps`
- Check database credentials in `.env`
- Restart services: `docker-compose restart`

### Issue: Bot doesn't respond in Slack
- Verify bot is invited to the channel
- Check `SLACK_CHANNEL` ID is correct
- Verify `SLACK_TOKEN` has correct permissions
- Check signing secret matches

## Maintenance

### View Logs
```bash
docker-compose logs -f main
docker-compose logs -f db
```

### Restart Services
```bash
docker-compose restart
```

### Update Code
```bash
git pull
docker-compose down
docker-compose up -d --build
```

### Backup Database
```bash
docker-compose exec db pg_dump -U postgres temp_email > backup.sql
```

### Clean Up Old Data
The scheduler automatically expires addresses after 24 hours. Database cleanup happens automatically.

## Security Recommendations

1. **Use HTTPS**: Set up a reverse proxy with SSL/TLS
2. **Firewall**: Only expose necessary ports
3. **Restrict Slack Channel**: Make the channel private
4. **Rate Limiting**: Consider adding rate limits to prevent abuse
5. **Monitor Logs**: Regularly check for suspicious activity
6. **Update Dependencies**: Keep Docker images and Go packages updated

## Architecture Notes

- **SMTP Server**: Listens on port 3000 (mapped to host port 25)
- **HTTP Server**: Listens on port 3001 (mapped to host port 3000 in compose, but actually runs on 3001)
- **Database**: PostgreSQL with persistent volume
- **Email Storage**: Raw emails stored in database, can be viewed via web interface
- **Expiration**: Addresses expire after 24 hours, users can reactivate

## Support

If you encounter issues:
1. Check the logs: `docker-compose logs -f`
2. Verify environment variables: `cat .env`
3. Test DNS configuration: `dig yourdomain.com MX`
4. Ensure all services are running: `docker-compose ps`
