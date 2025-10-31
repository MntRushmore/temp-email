# 🐛 Bug Fix: 550 Recipient Rejected

## Problem Identified

**Error:** `550 5.0.1 Recipient rejected`

**Root Cause:** Database insert was failing silently. The bot would show "wahoo!" with an address, but the address was never actually saved to the database. When emails arrived, the SMTP server couldn't find the address and rejected them.

## What Was Wrong

```go
// BEFORE (slackevents.go line 183):
email := db.Address{...}
db.DB.Create(&email)  // ❌ No error checking!
```

If `Create()` failed (database connection issue, timeout, constraint violation), the code would:
1. ✅ Show "wahoo! your address is xyz@rushilchopra.com" in Slack
2. ❌ But the address was NOT in the database
3. ❌ Emails to xyz@rushilchopra.com would be rejected

## The Fix

```go
// AFTER:
email := db.Address{...}
result := db.DB.Create(&email)
if result.Error != nil {
    // Log the error
    log.Printf("ERROR: Failed to create address %s: %v", address, result.Error)
    // Tell user it failed
    Client.PostMessage(ev.Channel, 
        slack.MsgOptionText("uh oh! something went wrong..."), 
        slack.MsgOptionTS(ev.TimeStamp))
    return  // Don't show "wahoo!" if it failed
}
// Log success
log.Printf("SUCCESS: Created address %s", address)
```

## New Features

### 1. Comprehensive Logging

**Address Creation Logs:**
```
SUCCESS: Created address abc123 for user U12345 (expires: 2024-01-15T12:00:00Z)
ERROR: Failed to create address xyz789 for user U67890: connection timeout
```

**Email Reception Logs:**
```
ACCEPT: Email received for abc123@rushilchopra.com from sender@example.com
REJECT: Address not found or expired: xyz789 (from: test@gmail.com)
```

### 2. Debug Endpoint

Check if an address exists:
```bash
curl https://mail.rushilchopra.com/api/check/abc123
```

Response if found:
```json
{
  "found": true,
  "id": "abc123",
  "created_at": "2024-01-15T11:30:00Z",
  "expires_at": "2024-01-16T11:30:00Z",
  "user": "U12345",
  "is_active": true
}
```

Response if not found:
```json
{
  "found": false,
  "error": "address not found or expired",
  "id": "xyz789"
}
```

### 3. User-Friendly Error Messages

Instead of silently failing, users now see:
```
uh oh! something went wrong creating that address. 
please try again or contact the admin. 
(error: database insert failed)
```

## Testing the Fix

### Step 1: Deploy the Update

On your production server:
```bash
cd /path/to/temp-email
git pull origin master
docker-compose down
docker-compose up -d --build
```

### Step 2: Have Friend Create Address Again

1. Friend types "gib email" in Slack
2. Check if bot responds with "wahoo!" or error message
3. If "wahoo!", note the address (e.g., `xyz789@rushilchopra.com`)

### Step 3: Verify Address in Database

Option A - Use debug endpoint:
```bash
curl https://mail.rushilchopra.com/api/check/xyz789
```

Option B - Check logs:
```bash
docker-compose logs web | grep "SUCCESS: Created"
```

Should show:
```
SUCCESS: Created address xyz789 for user U67890 (expires: 2024-01-15T12:00:00Z)
```

### Step 4: Test Email Delivery

Send test email to the address:
```bash
echo "Test message" | mail -s "Test" xyz789@rushilchopra.com
```

Check logs:
```bash
docker-compose logs smtp | tail -20
```

Should show:
```
ACCEPT: Email received for xyz789@rushilchopra.com from sender@example.com
```

If you see:
```
REJECT: Address not found or expired: xyz789
```

Then the address still isn't being created. Check for error logs:
```bash
docker-compose logs web | grep "ERROR: Failed to create"
```

## Troubleshooting

### If Addresses Still Fail to Create

Check logs for the actual error:
```bash
docker-compose logs web | grep "ERROR"
```

Common issues:

1. **Database Connection Lost:**
   ```
   ERROR: Failed to create address abc123: dial tcp: connection refused
   ```
   **Fix:** Check if database container is running: `docker-compose ps`

2. **Duplicate Key Error:**
   ```
   ERROR: Failed to create address abc123: duplicate key value violates unique constraint
   ```
   **Fix:** Very unlikely with random generation, but check if address already exists

3. **Permission Denied:**
   ```
   ERROR: Failed to create address abc123: permission denied
   ```
   **Fix:** Check database user permissions

### If Emails Still Bounce

1. **Check if address exists:**
   ```bash
   curl https://mail.rushilchopra.com/api/check/[address-id]
   ```

2. **Check SMTP logs:**
   ```bash
   docker-compose logs smtp | grep REJECT
   ```

3. **Check if address expired:**
   - Addresses expire after 24 hours by default
   - Check `expires_at` timestamp in debug endpoint response

## Expected Behavior After Fix

✅ **When creation succeeds:**
- Bot shows "wahoo!" message with address
- Address is saved to database
- Emails are received and posted to Slack thread
- Logs show: `SUCCESS: Created address...`

✅ **When creation fails:**
- Bot shows error message (NOT "wahoo!")
- No fake address is shown to user
- User knows to try again
- Logs show: `ERROR: Failed to create address...`

## Deployment Checklist

- [ ] Pull latest code: `git pull origin master`
- [ ] Rebuild containers: `docker-compose up -d --build`
- [ ] Test address creation: Have someone type "gib email"
- [ ] Verify in debug endpoint: `curl .../api/check/[address-id]`
- [ ] Test email delivery: Send test email
- [ ] Check logs: `docker-compose logs web | grep -E "SUCCESS|ERROR"`
- [ ] Monitor for 24 hours for any new issues

## Prevention

This bug was caused by **missing error handling**. To prevent similar issues:

1. ✅ Always check `.Error` on GORM operations
2. ✅ Log successes AND failures
3. ✅ Provide user feedback on errors
4. ✅ Add debug endpoints for critical operations
5. ✅ Monitor logs for error patterns

---

**Status:** Fix deployed in commit `6a35784`

**Next Steps:** Deploy to production and monitor logs for 24 hours to ensure addresses are being created successfully.
