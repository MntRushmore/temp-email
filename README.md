# Temp Email Service 📧

A temporary email issuance service for [Hack Club Slack](https://hackclub.com/slack) (or any Slack workspace).

Creates disposable 24-hour email addresses on demand, receives emails via SMTP, and posts them to Slack threads.

## Features

- 🚀 **On-Demand Email Addresses**: Users request temporary emails with "gib email" in Slack
- 📬 **Automatic Email Reception**: SMTP server receives emails and posts them to Slack threads
- 🌐 **Web Viewer**: View full HTML emails in browser via unique links
- ⏰ **24-Hour Expiration**: Addresses automatically expire after 24 hours
- 🔄 **Reactivation**: Users can reactivate expired addresses
- 🗄️ **Database Storage**: PostgreSQL stores addresses and emails
- 🐳 **Docker Ready**: Easy deployment with Docker Compose

## Quick Start

1. **Run the setup wizard:**
   ```bash
   ./setup.sh
   ```

2. **Check DNS configuration:**
   ```bash
   ./check-dns.sh
   ```

3. **Start the service:**
   ```bash
   docker-compose up -d
   ```

4. **Test in Slack:**
   - Post "gib email" in your configured channel
   - Send a test email to the generated address
   - Watch it appear in the Slack thread!

## Documentation

- **[SETUP.md](SETUP.md)** - Complete setup guide with detailed instructions
- **[setup.sh](setup.sh)** - Interactive configuration wizard
- **[check-dns.sh](check-dns.sh)** - DNS configuration checker
- **[troubleshoot.sh](troubleshoot.sh)** - Diagnostic tool for common issues

## Prerequisites

- Domain name with DNS access
- Slack workspace with admin privileges
- Server with Docker and Docker Compose
- Ports 25 (SMTP) and 3001 (HTTP) accessible

## Architecture

```
┌─────────────┐     SMTP (Port 25)     ┌──────────────┐
│   Sender    │───────────────────────>│ SMTP Server  │
└─────────────┘                        │  (Port 3000) │
                                       └──────┬───────┘
                                              │
                                              v
┌─────────────┐                        ┌──────────────┐
│    Slack    │<───────────────────────│  HTTP Server │
│   Channel   │    Webhooks/Events     │  (Port 3001) │
└─────────────┘                        └──────┬───────┘
                                              │
                                              v
                                       ┌──────────────┐
                                       │  PostgreSQL  │
                                       │   Database   │
                                       └──────────────┘
```

## Tech Stack

- **Go** - Main application language
- **SMTP Server** - Email reception (github.com/emersion/go-smtp)
- **Gin** - HTTP web framework
- **PostgreSQL** - Database storage
- **GORM** - ORM for database operations
- **Slack Go SDK** - Slack API integration
- **Docker** - Containerization

## Usage

### Requesting an Email
Post in your Slack channel:
```
gib email
```

The bot responds with a temporary email address:
```
wahoo! your temporary 24-hour email address is abc123@yourdomain.com

to stop receiving emails, delete your 'gib email' message.

i'll post emails in this thread ⬇️
```

### Receiving Emails
Emails sent to your temporary address automatically appear as replies in the thread with:
- Sender information
- Subject line
- Message body (converted from HTML to Slack markdown)
- Link to view full email in browser

### Deactivating
Delete your original "gib email" message to immediately deactivate the address.

### Reactivating
If your address expires, you can reactivate it for another 24 hours using the button in the expiration message.

## Configuration

Create a `.env` file with these variables:

```bash
# Slack Configuration
SLACK_TOKEN=xoxb-your-bot-token
SLACK_SIGNING_SECRET=your-signing-secret
SLACK_CHANNEL=C01234567

# Domain Configuration
DOMAIN=yourdomain.com
APP_DOMAIN=https://temp.yourdomain.com

# Database Configuration
DATABASE_URL=postgres://postgres:postgres@db:5432/temp_email
```

See [SETUP.md](SETUP.md) for detailed configuration instructions.

## Troubleshooting

Run the troubleshooting script:
```bash
./troubleshoot.sh
```

Common issues:
- **Not receiving emails?** Check DNS records with `./check-dns.sh`
- **Slack events not working?** Verify signing secret and event URL
- **Services won't start?** Check logs with `docker-compose logs -f`

See [SETUP.md](SETUP.md) for more troubleshooting tips.

## Development

### Local Development
```bash
# Install dependencies
go mod download

# Run locally (requires PostgreSQL)
export DATABASE_URL=postgres://user:pass@localhost:5432/temp_email
go run main.go
```

### Building
```bash
docker-compose build
```

### Logs
```bash
docker-compose logs -f
```

## Security

- Uses Slack signature verification for webhook security
- Anonymous SMTP (no authentication required)
- Rate limiting recommended for production
- Use HTTPS for web interface
- Keep Slack channel private

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

## License

See LICENSE file for details.

## Credits

Created for [Hack Club](https://hackclub.com/) - A community of high school hackers and makers.

## Support

- Check [SETUP.md](SETUP.md) for detailed setup instructions
- Run `./troubleshoot.sh` for diagnostic information
- Review `docker-compose logs` for error messages
- Open an issue for bugs or feature requests
