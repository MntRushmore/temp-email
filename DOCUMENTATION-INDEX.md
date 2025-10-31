# Documentation Index 📚

Complete documentation for the Temp Email Service.

## 📖 Documentation Files

### 🚀 [QUICK-START.md](QUICK-START.md) - **Start Here!**
**For**: First-time users who want to get started quickly  
**Time**: 30 minutes  
**Contents**:
- Step-by-step setup instructions
- Prerequisites checklist
- Common commands reference
- Quick troubleshooting tips
- Architecture overview

### 📘 [SETUP.md](SETUP.md) - Complete Setup Guide
**For**: Detailed setup with explanations  
**Time**: 45-60 minutes  
**Contents**:
- Comprehensive DNS configuration
- Detailed Slack app setup
- Environment variable explanations
- Server configuration
- Security recommendations
- Troubleshooting guide
- Maintenance procedures

### 🌐 [DOMAIN-SETUP.md](DOMAIN-SETUP.md) - DNS Configuration
**For**: Help with DNS records for various registrars  
**Contents**:
- Required DNS records explained
- Registrar-specific instructions (Cloudflare, Namecheap, GoDaddy, etc.)
- DNS verification tools
- Common DNS issues and solutions
- Subdomain vs root domain considerations

### 🔧 [README.md](README.md) - Project Overview
**For**: Understanding the project and quick reference  
**Contents**:
- Project description and features
- Quick start commands
- Architecture diagram
- Tech stack information
- Usage examples
- Development guide

## 🛠️ Interactive Scripts

### `./setup.sh` - Configuration Wizard
**Purpose**: Interactive setup and configuration  
**What it does**:
- Prompts for all required settings
- Validates configuration
- Creates `.env` file
- Offers to start services
- Provides DNS checklist

**Usage**:
```bash
./setup.sh
```

### `./check-dns.sh` - DNS Verification
**Purpose**: Verify DNS records are configured correctly  
**What it does**:
- Checks MX records
- Verifies A records
- Checks SPF records
- Displays current configuration
- Suggests fixes for issues

**Usage**:
```bash
./check-dns.sh
```

### `./troubleshoot.sh` - Diagnostic Tool
**Purpose**: Diagnose and fix common issues  
**What it does**:
- Checks configuration files
- Verifies Docker installation
- Tests service status
- Validates DNS setup
- Tests Slack API connection
- Shows recent logs
- Provides solutions for common problems

**Usage**:
```bash
./troubleshoot.sh
```

## 📋 Which Guide Should I Use?

### I'm new to this → Start with [QUICK-START.md](QUICK-START.md)
Get up and running in 30 minutes with step-by-step instructions.

### I need detailed explanations → Use [SETUP.md](SETUP.md)
Comprehensive guide with explanations for every step.

### I'm stuck on DNS → Check [DOMAIN-SETUP.md](DOMAIN-SETUP.md)
Registrar-specific instructions for DNS configuration.

### Something's not working → Run `./troubleshoot.sh`
Automated diagnostics for common issues.

### I want to understand the project → Read [README.md](README.md)
Architecture, features, and development information.

## 📊 Setup Flow

```
┌─────────────────────────────────────────────────────────┐
│                  START HERE                             │
│                                                         │
│  Do you have 30 minutes for quick setup?               │
│         │                                               │
│    ┌────┴────┐                                         │
│    │   YES   │  → QUICK-START.md → ./setup.sh          │
│    └─────────┘                                          │
│                                                         │
│    ┌─────────┐                                         │
│    │   NO    │  → Need more details?                   │
│    └────┬────┘                                          │
│         │                                               │
│         ├──→ General Setup → SETUP.md                  │
│         ├──→ DNS Help → DOMAIN-SETUP.md                │
│         └──→ Project Info → README.md                  │
│                                                         │
│  Having issues?                                         │
│         │                                               │
│         ├──→ DNS Problems → ./check-dns.sh             │
│         ├──→ Service Issues → ./troubleshoot.sh        │
│         └──→ General Help → SETUP.md (Troubleshooting) │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

## 🎯 Quick Reference by Task

### Initial Setup
1. **Configure DNS**: [DOMAIN-SETUP.md](DOMAIN-SETUP.md)
2. **Setup Slack App**: [SETUP.md](SETUP.md#step-2-create-slack-app)
3. **Run Configuration**: `./setup.sh`
4. **Start Service**: `docker-compose up -d`

### Verification
- **Check DNS**: `./check-dns.sh`
- **View Logs**: `docker-compose logs -f`
- **Test Service**: Post "gib email" in Slack

### Troubleshooting
- **Run Diagnostics**: `./troubleshoot.sh`
- **Check Logs**: `docker-compose logs -f main`
- **Restart Services**: `docker-compose restart`

### Maintenance
- **View Status**: `docker-compose ps`
- **Update Code**: `git pull && docker-compose up -d --build`
- **Backup Database**: See [QUICK-START.md](QUICK-START.md#production-recommendations)

## 📝 Environment Variables

All configuration is done via `.env` file. Use `./setup.sh` to create it, or manually:

```bash
SLACK_TOKEN=xoxb-...              # Required: Slack Bot Token
SLACK_SIGNING_SECRET=...          # Required: Slack Signing Secret
SLACK_CHANNEL=C01234567           # Required: Slack Channel ID
DOMAIN=yourdomain.com             # Required: Email domain
APP_DOMAIN=https://temp.yourdomain.com  # Required: Web interface URL
DATABASE_URL=postgres://...       # Auto-configured by docker-compose
```

See [SETUP.md](SETUP.md#step-3-configure-environment-variables) for details.

## 🏗️ Architecture Components

### Services (via Docker Compose)
- **main**: Go application (SMTP + HTTP server)
- **db**: PostgreSQL database

### Ports
- **25**: SMTP server (email reception)
- **3000**: Internal SMTP port (mapped to 25)
- **3001**: HTTP server (webhooks + web viewer)

### Data Flow
1. Email arrives → SMTP server (port 25)
2. Email parsed → Stored in PostgreSQL
3. Notification → Posted to Slack via API
4. User clicks link → HTTP server serves email (port 3001)

See [README.md](README.md#architecture) for diagram.

## 🔒 Security Notes

### Essential for Production
- ✅ Use HTTPS (reverse proxy with SSL/TLS)
- ✅ Firewall rules (only ports 25, 80, 443)
- ✅ Private Slack channel
- ✅ Keep Docker images updated
- ✅ Regular backups

### Recommended
- ✅ SPF records
- ✅ Rate limiting
- ✅ Monitoring
- ✅ Log rotation
- ✅ 2FA on domain registrar

See [SETUP.md](SETUP.md#security-recommendations) for details.

## 🐛 Common Issues

| Issue | Check | Solution |
|-------|-------|----------|
| Services won't start | `./troubleshoot.sh` | Check logs, restart Docker |
| Not receiving emails | `./check-dns.sh` | Fix DNS records, check port 25 |
| Slack events fail | Logs | Verify signing secret, check URL |
| Bot doesn't respond | Config | Check token, channel ID, permissions |
| DNS not resolving | `dig` command | Wait for propagation (15-60 min) |

See [SETUP.md](SETUP.md#troubleshooting) for comprehensive troubleshooting.

## 📦 Files Overview

```
temp-email/
├── README.md                 # Project overview
├── QUICK-START.md           # Fast setup guide
├── SETUP.md                 # Detailed setup guide
├── DOMAIN-SETUP.md          # DNS configuration help
├── DOCUMENTATION-INDEX.md   # This file
│
├── setup.sh                 # Interactive configuration wizard
├── check-dns.sh            # DNS verification tool
├── troubleshoot.sh         # Diagnostic tool
│
├── docker-compose.yml      # Docker services configuration
├── Dockerfile              # Docker image definition
├── .env.example            # Environment variables template
│
├── main.go                 # Main application entry point
├── go.mod                  # Go dependencies
├── go.sum                  # Go dependency checksums
│
└── pkg/                    # Application packages
    ├── db/                 # Database models and connection
    ├── schedule/           # Email expiration scheduler
    ├── slackevents/        # Slack event handling
    └── util/               # Utility functions
```

## 🎓 Learning Path

### Beginner
1. Read [README.md](README.md) to understand the project
2. Follow [QUICK-START.md](QUICK-START.md) for setup
3. Use `./setup.sh` for easy configuration
4. Test with "gib email" in Slack

### Intermediate
1. Read [SETUP.md](SETUP.md) for detailed understanding
2. Manually configure DNS using [DOMAIN-SETUP.md](DOMAIN-SETUP.md)
3. Setup HTTPS with reverse proxy
4. Configure monitoring and backups

### Advanced
1. Review source code (main.go, pkg/)
2. Customize email handling
3. Add new features
4. Deploy to production with full monitoring

## 🆘 Getting Help

1. **Check documentation**: Start with relevant guide above
2. **Run diagnostics**: `./troubleshoot.sh` for automated checks
3. **Check DNS**: `./check-dns.sh` for DNS issues
4. **Review logs**: `docker-compose logs -f`
5. **Search issues**: Check GitHub issues for similar problems
6. **Ask for help**: Open a new issue with diagnostic output

## 📚 Additional Resources

- **Go SMTP Library**: https://github.com/emersion/go-smtp
- **Slack Go SDK**: https://github.com/slack-go/slack
- **Docker Compose**: https://docs.docker.com/compose/
- **PostgreSQL**: https://www.postgresql.org/docs/
- **Gin Framework**: https://gin-gonic.com/docs/

## ✨ Tips & Best Practices

1. **Start with quick-start**: Don't overwhelm yourself with details initially
2. **Run scripts**: Use provided scripts for common tasks
3. **Test DNS early**: Configure and verify DNS before setting up services
4. **Use HTTPS**: Always use HTTPS in production
5. **Monitor logs**: Regularly check logs for issues
6. **Backup regularly**: Don't lose your data
7. **Keep updated**: Update Docker images and dependencies
8. **Document changes**: Keep notes of customizations

---

**Need help?** Start with the appropriate guide above, or run `./troubleshoot.sh` for automated diagnostics.

**Ready to begin?** → [QUICK-START.md](QUICK-START.md)
