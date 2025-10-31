# Quick Start Guide 🚀

Get your temp email service running in under 30 minutes!

## Prerequisites Checklist

Before you begin, make sure you have:

- [ ] A domain name (e.g., `example.com`)
- [ ] Server with Docker installed
- [ ] Server with public IP address
- [ ] Access to domain DNS settings
- [ ] Slack workspace admin access
- [ ] Ports 25 and 3001 open on server

## Step-by-Step Setup

### Step 1: Configure DNS (5-10 minutes)

Add these DNS records to your domain:

```
MX    @     mail.yourdomain.com      Priority: 10
A     mail  YOUR_SERVER_IP
A     temp  YOUR_SERVER_IP
TXT   @     v=spf1 mx ~all
```

📖 **Detailed guide**: [DOMAIN-SETUP.md](DOMAIN-SETUP.md)

⏱️ **Wait 15 minutes** for DNS propagation, then verify:
```bash
./check-dns.sh
```

---

### Step 2: Create Slack App (10-15 minutes)

1. **Go to**: https://api.slack.com/apps
2. **Create new app** → "From scratch"
3. **Add OAuth scopes**:
   - `channels:history`
   - `channels:read`
   - `chat:write`
   - `reactions:write`
4. **Install to workspace**
5. **Copy Bot Token** (starts with `xoxb-`)
6. **Copy Signing Secret**
7. **Create a channel** and invite the bot
8. **Copy Channel ID**

📖 **Detailed guide**: [SETUP.md](SETUP.md)

---

### Step 3: Run Setup Wizard (2 minutes)

```bash
cd /home/user/webapp
./setup.sh
```

This interactive script will:
- ✅ Configure environment variables
- ✅ Validate your settings
- ✅ Optionally start the service

---

### Step 4: Start the Service (1 minute)

```bash
docker-compose up -d
```

Check if services are running:
```bash
docker-compose ps
```

View logs:
```bash
docker-compose logs -f
```

---

### Step 5: Configure Slack Events (2 minutes)

1. Go back to your Slack app settings
2. Navigate to **Event Subscriptions**
3. Enable events
4. Set Request URL to: `https://temp.yourdomain.com/slack/events`
5. Subscribe to bot event: `message.channels`
6. Save changes

---

### Step 6: Test It! (1 minute)

1. Go to your Slack channel
2. Post: `gib email`
3. Bot should respond with a temporary email address
4. Send an email to that address
5. Watch it appear in the Slack thread! 🎉

---

## What You Should See

### In Slack (after "gib email"):
```
wahoo! your temporary 24-hour email address is abc123@yourdomain.com

to stop receiving emails, delete your 'gib email' message.

i'll post emails in this thread ⬇️
```

### In Logs (docker-compose logs):
```
Starting up SMTP server...
Starting up HTTP server...
[GIN-debug] Listening and serving HTTP on :3001
```

---

## Troubleshooting

### Services won't start
```bash
./troubleshoot.sh
docker-compose logs
```

### Not receiving emails
```bash
./check-dns.sh
docker-compose logs main
```

### Slack events not working
- Verify the Request URL in Slack app settings
- Check the signing secret in `.env`
- Ensure port 3001 is accessible from the internet

### Bot doesn't respond
- Verify bot is invited to the channel
- Check `SLACK_CHANNEL` ID is correct
- Confirm OAuth token has correct permissions

---

## Common Commands

```bash
# View logs
docker-compose logs -f

# Restart services
docker-compose restart

# Stop services
docker-compose down

# Rebuild and restart
docker-compose up -d --build

# Check DNS
./check-dns.sh

# Run diagnostics
./troubleshoot.sh

# View environment variables
cat .env
```

---

## Architecture Overview

```
┌──────────────────────────────────────────────────────────┐
│                     Your Server                          │
│                                                          │
│  ┌────────────┐  Port 25    ┌──────────────┐           │
│  │   SMTP     │◄────────────│ Email Sender │           │
│  │  Server    │             └──────────────┘           │
│  │ (Port 3000)│                                         │
│  └─────┬──────┘                                         │
│        │                                                │
│        │  Store in DB                                   │
│        ▼                                                │
│  ┌────────────┐             ┌──────────────┐           │
│  │ PostgreSQL │             │     HTTP     │           │
│  │  Database  │             │    Server    │◄──────┐   │
│  └────────────┘             │  (Port 3001) │       │   │
│                             └──────┬───────┘       │   │
│                                    │               │   │
└────────────────────────────────────┼───────────────┼───┘
                                     │               │
                              Webhooks        View Emails
                                     │               │
                                     ▼               │
                              ┌──────────┐          │
                              │  Slack   │          │
                              │ Channel  │          │
                              └──────────┘          │
                                                     │
                              ┌──────────┐          │
                              │ Browser  │──────────┘
                              └──────────┘
```

---

## Environment Variables Reference

```bash
# Slack Configuration
SLACK_TOKEN=xoxb-...           # Bot User OAuth Token
SLACK_SIGNING_SECRET=...       # App signing secret
SLACK_CHANNEL=C01234567        # Channel ID

# Domain Configuration
DOMAIN=yourdomain.com          # Email domain (@yourdomain.com)
APP_DOMAIN=https://temp.yourdomain.com  # Web interface URL

# Database (auto-configured by docker-compose)
DATABASE_URL=postgres://postgres:postgres@db:5432/temp_email
```

---

## Security Checklist

- [ ] Use HTTPS for web interface (setup reverse proxy)
- [ ] Keep Slack channel private
- [ ] Use strong passwords for database
- [ ] Keep Docker images updated
- [ ] Monitor logs for suspicious activity
- [ ] Restrict firewall rules to necessary ports
- [ ] Enable 2FA on domain registrar
- [ ] Configure SPF records
- [ ] Regularly backup database

---

## Production Recommendations

### 1. Setup HTTPS with Reverse Proxy

**Using Caddy** (easiest):
```bash
# Install Caddy
curl -s https://caddyserver.com/api/download | sh

# Create Caddyfile
cat > Caddyfile << EOF
temp.yourdomain.com {
    reverse_proxy localhost:3001
}
EOF

# Start Caddy
caddy run
```

**Using Nginx**:
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

### 2. Setup Automatic Backups

```bash
# Create backup script
cat > backup.sh << 'EOF'
#!/bin/bash
DATE=$(date +%Y%m%d_%H%M%S)
docker-compose exec -T db pg_dump -U postgres temp_email > backup_$DATE.sql
EOF

chmod +x backup.sh

# Add to crontab (daily at 2am)
echo "0 2 * * * cd /home/user/webapp && ./backup.sh" | crontab -
```

### 3. Setup Monitoring

```bash
# Install monitoring script
cat > monitor.sh << 'EOF'
#!/bin/bash
if ! docker-compose ps | grep -q "Up"; then
    echo "Services are down! Restarting..."
    docker-compose up -d
    # Send alert (setup email/Slack notification)
fi
EOF

chmod +x monitor.sh

# Add to crontab (every 5 minutes)
echo "*/5 * * * * cd /home/user/webapp && ./monitor.sh" | crontab -
```

---

## Next Steps

✅ **Service is running?** Great! Here's what to do next:

1. **Test thoroughly**: Send various types of emails (plain text, HTML, attachments)
2. **Monitor logs**: Watch for errors or issues
3. **Setup HTTPS**: Use a reverse proxy for production
4. **Configure backups**: Don't lose your data
5. **Share with team**: Invite others to use the service
6. **Customize**: Modify the code to fit your needs

---

## Getting Help

- 📖 **Full documentation**: [SETUP.md](SETUP.md)
- 🌐 **Domain setup**: [DOMAIN-SETUP.md](DOMAIN-SETUP.md)
- 🔧 **Diagnostics**: Run `./troubleshoot.sh`
- 📊 **Check DNS**: Run `./check-dns.sh`
- 📝 **View logs**: `docker-compose logs -f`

---

## FAQ

**Q: Can I use this for production?**  
A: Yes, but add HTTPS, monitoring, and backups first.

**Q: How long do emails stay stored?**  
A: Addresses expire after 24 hours, but emails remain in the database.

**Q: Can I extend the 24-hour limit?**  
A: Yes, modify the expiration time in the code (`24 * time.Hour`).

**Q: Does this support attachments?**  
A: Yes, but they're stored in the database. Consider file size limits.

**Q: Can I use multiple domains?**  
A: Currently supports one domain. Code modification needed for multiple domains.

**Q: Is this secure?**  
A: It uses Slack signature verification. Add HTTPS and firewall rules for production.

**Q: Can I self-host this?**  
A: Yes! That's exactly what this guide is for.

---

🎉 **Congratulations!** You've successfully set up your temporary email service!

If you found this useful, consider starring the repository and sharing it with others.
