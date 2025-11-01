# 📧 Mailgun Integration Setup Guide

## Why Mailgun?

Your SMTP server has been unreliable due to:
- Database connection issues causing silent failures
- Port 25 blocking by ISPs
- SPF/DKIM/DMARC complexity
- IP reputation problems

**Mailgun solves all of this** by handling email receiving for you and posting to a webhook.

## Your Mailgun Details

- **Sandbox Domain**: `sandbox8822e8e06d904455a74c0d9d6375ecd3.mailgun.org`
- **API Key**: `[Get from Mailgun Dashboard → API Keys]`
- **Base URL**: `https://api.mailgun.net`

Note: Never commit API keys to git! Add them only to your `.env` file on the server.

## How It Works

```
Email sent to xyz@rushilchopra.com
    ↓
[Your DNS MX records point to Mailgun]
    ↓
Mailgun receives email
    ↓
Mailgun POSTs to: https://mail.rushilchopra.com/webhook/mailgun
    ↓
Your Go app processes it
    ↓
Posted to Slack & visible in dashboard
```

## Step 1: Add Mailgun Environment Variables

Edit your `.env` file on the server:

```bash
cd ~/temp-email
nano .env
```

Add these lines:

```bash
MAILGUN_API_KEY=your-api-key-here
MAILGUN_DOMAIN=sandbox8822e8e06d904455a74c0d9d6375ecd3.mailgun.org
MAILGUN_SIGNING_KEY=
```

Note: Leave `MAILGUN_SIGNING_KEY` empty for now. We'll get it from Mailgun after setup.

## Step 2: Deploy the New Code

```bash
cd ~/temp-email
git pull origin master
docker-compose build --no-cache
docker-compose down
docker-compose up -d
```

Verify webhook endpoint is running:
```bash
curl http://localhost:3000/webhook/mailgun
# Should return: {"error":"Invalid timestamp"} or similar (means it's working)
```

## Step 3: Configure Mailgun Routes

### Option A: Using Mailgun Dashboard (Easiest)

1. Go to https://app.mailgun.com/app/sending/domains
2. Click on your sandbox domain
3. Go to **"Routes"** tab
4. Click **"Create Route"**

**Route Configuration:**
- **Expression Type**: Match Recipient
- **Recipient**: `.*@sandbox8822e8e06d904455a74c0d9d6375ecd3.mailgun.org`
- **Actions**: Forward to URL
- **URL**: `https://mail.rushilchopra.com/webhook/mailgun`
- **Priority**: 0 (highest)
- **Description**: Forward all emails to temp-email webhook

Click **"Create Route"**

### Option B: Using API (Alternative)

```bash
curl -X POST https://api.mailgun.net/v3/routes \
    -u "api:YOUR_API_KEY_HERE" \
    -F "priority=0" \
    -F "expression=match_recipient('.*@sandbox8822e8e06d904455a74c0d9d6375ecd3.mailgun.org')" \
    -F "action=forward('https://mail.rushilchopra.com/webhook/mailgun')" \
    -F "description=Temp Email Webhook"
```

## Step 4: Get Webhook Signing Key

1. Go to https://app.mailgun.com/app/account/security/api_keys
2. Find **"HTTP webhook signing key"**
3. Copy it
4. Add to your `.env`:

```bash
MAILGUN_SIGNING_KEY=your-signing-key-here
```

5. Restart:
```bash
docker-compose restart main
```

## Step 5: Test Email Delivery

### Test with Mailgun Testing Tool

1. Go to Mailgun Dashboard → Your Domain → Sending
2. Click **"Send a test message"**
3. Create a test address in your dashboard first (e.g., `test123`)
4. Send test email to: `test123@sandbox8822e8e06d904455a74c0d9d6375ecd3.mailgun.org`

### Test with Real Email

Send an email from your Gmail/Outlook to:
```
test123@sandbox8822e8e06d904455a74c0d9d6375ecd3.mailgun.org
```

### Check Logs

```bash
docker-compose logs main | grep -E "Mailgun webhook|ACCEPT|REJECT"
```

You should see:
```
Mailgun webhook received: to=test123@sandbox... from=sender@gmail.com subject=Test
ACCEPT: Email received for test123@sandbox... from sender@gmail.com
```

## Step 6: Use Your Custom Domain (Optional but Recommended)

Currently using sandbox domain: `sandbox8822e8e06d904455a74c0d9d6375ecd3.mailgun.org`

To use `rushilchopra.com`:

### 6.1: Verify Your Domain in Mailgun

1. Go to https://app.mailgun.com/app/sending/domains
2. Click **"Add New Domain"**
3. Enter: `mail.rushilchopra.com` (or just `rushilchopra.com`)
4. Mailgun will show DNS records you need to add

### 6.2: Add DNS Records

Add these to your DNS provider (where rushilchopra.com is hosted):

**MX Records:**
```
Priority  Host                  Value
10        mail.rushilchopra.com  mxa.mailgun.org
10        mail.rushilchopra.com  mxb.mailgun.org
```

**TXT Records (for verification):**
```
Host: mail.rushilchopra.com
Value: v=spf1 include:mailgun.org ~all
```

```
Host: mx._domainkey.mail.rushilchopra.com  
Value: [Mailgun will provide this - copy from their dashboard]
```

### 6.3: Update Environment Variables

After domain is verified:

```bash
MAILGUN_DOMAIN=mail.rushilchopra.com
DOMAIN=mail.rushilchopra.com
```

### 6.4: Update Mailgun Route

Change route expression to:
```
match_recipient('.*@mail.rushilchopra.com')
```

Then restart:
```bash
docker-compose restart main
```

## Step 7: Disable SMTP Server (Optional)

Once Mailgun is working, you can disable the SMTP server:

Edit `main.go` and comment out the SMTP server:

```go
// backend := Backend{}
// server := smtp.NewServer(backend)
// server.Addr = ":3000"
// server.Domain = os.Getenv("DOMAIN")
// 
// // Spin up an SMTP server in a goroutine
// go func() {
//     log.Println("Starting up SMTP server...")
//     err := server.ListenAndServe()
//     if err != nil {
//         log.Fatal(err)
//     }
// }()
```

Update `docker-compose.yml` to remove port 25:

```yaml
ports:
  - "3000:3001"
  # - "25:3000"  # Remove this line
```

Rebuild:
```bash
docker-compose build --no-cache && docker-compose down && docker-compose up -d
```

## Troubleshooting

### Webhook Not Receiving Emails

**Check Mailgun logs:**
1. Go to https://app.mailgun.com/app/logs
2. Look for delivery attempts
3. Check for errors

**Check your app logs:**
```bash
docker-compose logs main --tail=100 | grep -i mailgun
```

**Test webhook manually:**
```bash
curl -X POST https://mail.rushilchopra.com/webhook/mailgun \
  -F "timestamp=$(date +%s)" \
  -F "token=test" \
  -F "signature=invalid" \
  -F "recipient=test@sandbox8822e8e06d904455a74c0d9d6375ecd3.mailgun.org" \
  -F "sender=test@example.com" \
  -F "subject=Test Subject" \
  -F "body-plain=Test body"
```

### "Invalid Signature" Error

- Make sure `MAILGUN_SIGNING_KEY` is set in `.env`
- Restart after adding: `docker-compose restart main`
- For testing, temporarily comment out signature verification

### Address Not Found

- Create address first in dashboard or via Slack
- Check database: `docker-compose exec db psql -U postgres -d temp_email -c "SELECT * FROM addresses WHERE id='test123';"`
- Check logs for REJECT messages

### Emails Not Appearing in Slack

- Address must be created via Slack (has timestamp)
- Dashboard-created addresses won't post to Slack automatically
- Check Slack token is valid: `echo $SLACK_TOKEN`

## Benefits of Mailgun

✅ **No more 550 errors** - Mailgun handles all SMTP complexity
✅ **Reliable delivery** - 99.9% uptime SLA
✅ **No port 25 issues** - All via HTTPS webhooks
✅ **Better logging** - See delivery attempts in Mailgun dashboard
✅ **Automatic retries** - Mailgun retries failed webhook deliveries
✅ **Spam filtering** - Built-in spam protection
✅ **Email parsing** - Mailgun parses HTML, attachments, etc.
✅ **Scalable** - Handle thousands of emails/month on free tier

## Sandbox vs Production Domain

**Sandbox Domain (Current):**
- ✅ Free forever
- ✅ No verification needed
- ❌ Can only send to authorized recipients
- ❌ Ugly domain name
- ✅ Perfect for testing

**Custom Domain (Recommended):**
- ✅ Professional (`xyz@rushilchopra.com`)
- ✅ No sending restrictions
- ✅ Your branding
- ⚠️  Requires DNS setup
- ✅ Still free tier (5,000 emails/month)

## Next Steps

1. ✅ Deploy code (Step 2)
2. ✅ Configure Mailgun route (Step 3)
3. ✅ Add signing key (Step 4)
4. ✅ Test email delivery (Step 5)
5. 🎯 Switch to custom domain (Step 6) - **Recommended!**
6. 🔧 Disable SMTP server (Step 7) - Optional

---

**Status**: Ready to deploy
**Estimated Setup Time**: 15 minutes
**Difficulty**: Easy

Once Mailgun is working, **all your 550 errors will disappear!** 🎉
