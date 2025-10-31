# Domain Setup Quick Reference

This guide provides specific DNS configuration examples for popular domain registrars.

## What You Need

- **Your domain name** (e.g., `example.com`)
- **Your server's IP address** (e.g., `203.0.113.42`)
- **Your web subdomain** (e.g., `temp.example.com`)

## Required DNS Records

### 1. MX Record (Mail Exchange)
Tells email servers where to deliver emails for your domain.

```
Type: MX
Name: @ (or your domain root)
Priority: 10
Value: mail.yourdomain.com
TTL: 3600 (or Auto)
```

### 2. A Record for Mail Server
Points the mail server hostname to your server's IP.

```
Type: A
Name: mail
Value: YOUR_SERVER_IP (e.g., 203.0.113.42)
TTL: 3600 (or Auto)
```

### 3. A Record for Web Interface
Points the web interface to your server's IP.

```
Type: A
Name: temp (or your chosen subdomain)
Value: YOUR_SERVER_IP (e.g., 203.0.113.42)
TTL: 3600 (or Auto)
```

### 4. SPF Record (Optional but Recommended)
Helps prevent email spoofing.

```
Type: TXT
Name: @ (or your domain root)
Value: v=spf1 mx ~all
TTL: 3600 (or Auto)
```

## Example Configuration

Let's say you have:
- **Domain**: `tempmail.example.com`
- **Server IP**: `203.0.113.42`
- **Web interface**: `temp.tempmail.example.com`

Your DNS records would be:

| Type | Name | Value | Priority | TTL |
|------|------|-------|----------|-----|
| MX | @ | mail.tempmail.example.com | 10 | 3600 |
| A | mail | 203.0.113.42 | - | 3600 |
| A | temp | 203.0.113.42 | - | 3600 |
| TXT | @ | v=spf1 mx ~all | - | 3600 |

## Popular Domain Registrars

### Cloudflare

1. Log in to Cloudflare Dashboard
2. Select your domain
3. Go to **DNS** tab
4. Click **Add record** for each entry:

**MX Record:**
- Type: `MX`
- Name: `@`
- Mail server: `mail.yourdomain.com`
- Priority: `10`
- Proxy status: DNS only (gray cloud)

**A Records:**
- Type: `A`
- Name: `mail` (or `temp`)
- IPv4 address: Your server IP
- Proxy status: DNS only (gray cloud) for mail, optional for temp

⚠️ **Important**: Turn off proxy (orange cloud) for the mail subdomain!

### Namecheap

1. Log in to Namecheap account
2. Go to **Domain List** → Select domain → **Advanced DNS**
3. Click **Add New Record** for each entry:

**MX Record:**
- Type: `MX Record`
- Host: `@`
- Value: `mail.yourdomain.com`
- Priority: `10`

**A Records:**
- Type: `A Record`
- Host: `mail` (or `temp`)
- Value: Your server IP

### GoDaddy

1. Log in to GoDaddy account
2. Go to **My Products** → **DNS**
3. Click **Add** for each record:

**MX Record:**
- Type: `MX`
- Host: `@`
- Points to: `mail.yourdomain.com`
- Priority: `10`

**A Records:**
- Type: `A`
- Host: `mail` (or `temp`)
- Points to: Your server IP

### Google Domains (Now Squarespace Domains)

1. Log in to your account
2. Select domain → **DNS**
3. Add custom resource records:

**MX Record:**
- Resource Name: `@`
- Resource Type: `MX`
- TTL: `1H`
- Priority: `10`
- Data: `mail.yourdomain.com`

**A Records:**
- Resource Name: `mail` (or `temp`)
- Resource Type: `A`
- TTL: `1H`
- Data: Your server IP

### AWS Route 53

1. Go to Route 53 Console
2. Select your hosted zone
3. Click **Create record**

**MX Record:**
- Record name: (leave blank for root)
- Record type: `MX`
- Value: `10 mail.yourdomain.com`
- TTL: `300`

**A Records:**
- Record name: `mail` (or `temp`)
- Record type: `A`
- Value: Your server IP
- TTL: `300`

### DigitalOcean

1. Go to **Networking** → **Domains**
2. Select your domain
3. Add each record:

**MX Record:**
- Hostname: `@`
- Will direct to: `mail.yourdomain.com`
- Priority: `10`

**A Records:**
- Hostname: `mail` (or `temp`)
- Will direct to: Your server IP

## Verification

After configuring DNS records, verify them:

### Using Command Line

```bash
# Check MX records
dig yourdomain.com MX

# Check A records
dig mail.yourdomain.com A
dig temp.yourdomain.com A

# Check SPF record
dig yourdomain.com TXT
```

### Using Online Tools

- **MXToolbox**: https://mxtoolbox.com/
- **DNS Checker**: https://dnschecker.org/
- **What's My DNS**: https://www.whatsmydns.net/

### Using the Included Script

```bash
./check-dns.sh
```

## Propagation Time

DNS changes can take time to propagate:
- **Typical**: 5-15 minutes
- **Maximum**: 24-48 hours (rare)
- **Most registrars**: 15-60 minutes

## Common Issues

### Issue: MX record not found
**Solution**: Double-check the hostname is exactly `mail.yourdomain.com` with no trailing dot (unless required by your registrar).

### Issue: A record not resolving
**Solution**: Ensure you didn't include `http://` or `https://` in the value field.

### Issue: Cloudflare proxy issues
**Solution**: Disable proxy (orange cloud → gray cloud) for mail subdomain. MX records cannot be proxied.

### Issue: Record type not available
**Solution**: Some registrars call MX records "Mail" or "Mail Exchange". A records might be called "Host".

### Issue: Priority field missing
**Solution**: Some registrars automatically set priority to 10. If you have multiple MX records, lower numbers = higher priority.

## Subdomain vs Root Domain

### Using Root Domain (example.com)
```
DOMAIN=example.com
APP_DOMAIN=https://temp.example.com
```

Email format: `abc123@example.com`

### Using Subdomain (temp.example.com)
```
DOMAIN=temp.example.com
APP_DOMAIN=https://viewer.temp.example.com
```

Email format: `abc123@temp.example.com`

Both work! Choose based on your preference.

## Multiple Domains

You can configure multiple domains by:
1. Adding MX records for each domain pointing to your server
2. Adding the domains to your `.env` file (requires code modification for multi-domain support)

For a single domain setup, just configure one domain as described above.

## Testing Your Setup

After DNS propagation:

1. **Test MX record resolution:**
   ```bash
   host -t MX yourdomain.com
   ```

2. **Test SMTP connectivity:**
   ```bash
   telnet mail.yourdomain.com 25
   ```
   Type `quit` to exit.

3. **Send test email:**
   ```bash
   echo "Test" | mail -s "Test" test@yourdomain.com
   ```

4. **Check service logs:**
   ```bash
   docker-compose logs -f main
   ```

## Need Help?

If you're stuck:
1. Run `./check-dns.sh` to verify DNS configuration
2. Run `./troubleshoot.sh` to diagnose issues
3. Check your registrar's documentation for DNS management
4. Wait 30 minutes for DNS propagation
5. Verify your server's firewall allows port 25 (SMTP)

## Security Notes

- **Keep DNS records up to date** if you change servers
- **Use strong passwords** for domain registrar account
- **Enable 2FA** on your domain registrar account
- **Monitor DNS changes** - some registrars offer change notifications
- **Consider DNSSEC** for additional security (advanced)

## Next Steps

After DNS is configured:
1. Wait for propagation (check with `./check-dns.sh`)
2. Continue with the main setup in [SETUP.md](SETUP.md)
3. Configure your Slack app
4. Start the service with `docker-compose up -d`
