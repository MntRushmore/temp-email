# Email Address Bounce Diagnostics

## Issue Report
User's friend creates emails via "gib email" in Slack but emails bounce.

## Code Analysis
✅ **No user discrimination in code**:
- Address creation (slackevents.go:175-183): Stores `ev.User` but doesn't filter
- Email receiving (main.go:54): Only checks `id` and `expires_at`, NOT user
- All addresses are treated equally

## Potential Causes

### 1. DNS/MX Record Issue ❌
**Most Likely Cause**: Email servers can't find or connect to your SMTP server.

**Test**: Run on your production server:
```bash
# Check MX records
dig MX rushilchopra.com

# Should show something like:
# rushilchopra.com.  300  IN  MX  10 mail.rushilchopra.com.
```

**Test sending**: From command line on server:
```bash
# Test SMTP connection
telnet rushilchopra.com 25
# Or
nc -zv rushilchopra.com 25
```

### 2. SPF/DKIM/DMARC Records ⚠️
Your domain might be blocking incoming emails due to sender policy.

**Check**: Look at bounce message - does it say:
- "550 SPF check failed"
- "DMARC policy rejection"

### 3. Firewall/Port Blocking 🔥
Port 25 (SMTP) might be blocked for certain IP ranges.

**Test**: Have your friend check their ISP/network:
- Many residential ISPs block outbound port 25
- Corporate networks often block SMTP

### 4. Email Provider Rate Limiting 📧
Gmail/Outlook might be rate-limiting or spam-filtering.

**Symptoms**:
- First email works, subsequent ones bounce
- Temporary vs permanent bounce
- "450 Try again later" vs "550 Rejected"

### 5. Address Format Issues 🔤
Check if friend's addresses look different:
- Correct: `abc123@rushilchopra.com`
- Wrong: `abc123@rushilchopra.com.` (trailing dot)
- Wrong: `abc123@RUSHILCHOPRA.COM` (case sensitivity - shouldn't matter but test)

## Diagnostic Steps

### Step 1: Identify the exact error
Ask your friend to screenshot or copy the EXACT bounce message. Look for:
```
550 5.1.1 User unknown
550 5.7.1 Relay access denied
451 4.4.1 Unable to connect
550 5.7.26 SPF check failed
```

### Step 2: Test with same address
1. Friend creates address via "gib email" → gets `xyz789@rushilchopra.com`
2. YOU send email to `xyz789@rushilchopra.com`
3. If it works → Problem is with friend's email provider/network
4. If it doesn't work → Problem is with address creation (unlikely based on code)

### Step 3: Check server logs
On your production server:
```bash
# Check recent SMTP logs
docker-compose logs smtp --tail=100 | grep -i "xyz789"

# Check for connection attempts
docker-compose logs smtp | grep -i "connection"

# Check for errors
docker-compose logs smtp | grep -i "error"
```

### Step 4: Compare addresses in database
```bash
# Run debug script on friend's address
./debug-email.sh xyz789

# Run debug script on your address
./debug-email.sh abc123

# Compare results - both should show:
# - is_active: true
# - hours_remaining: ~24
# - created_at: timestamp
```

## Quick Fix Attempts

### Fix 1: SPF Record
Add to DNS:
```
rushilchopra.com.  TXT  "v=spf1 a mx ~all"
```

### Fix 2: Check Docker/SMTP Server
```bash
# Ensure SMTP server is listening on public interface
docker-compose exec smtp netstat -tlnp | grep 3000
# Should show: 0.0.0.0:3000

# Check if port 25 is exposed
docker-compose ps
```

### Fix 3: Test Email Delivery Manually
```bash
# From external server, test SMTP connection:
telnet rushilchopra.com 25
EHLO test.com
MAIL FROM:<test@example.com>
RCPT TO:<xyz789@rushilchopra.com>
DATA
Subject: Test
This is a test
.
QUIT
```

## Next Steps

1. **Get the bounce message** - This will tell us exactly what's wrong
2. **Test with same address** - Confirms if it's a code issue or network issue
3. **Check DNS records** - Verify MX records are correct
4. **Review SMTP logs** - Look for connection attempts and errors

## Conclusion

Based on code review: **This is NOT a code issue**. All addresses are created and handled identically regardless of which Slack user creates them.

**Most likely cause**: Network/DNS/firewall issue preventing emails from reaching your SMTP server.

**Next action needed**: Get the exact bounce message from your friend's email provider.
