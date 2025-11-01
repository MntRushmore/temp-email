# 🔧 Fix Deployment Guide - 550 Recipient Rejected

## Critical Bug Fixed

**Issue:** Both Slack bot AND dashboard address creation were failing silently due to missing error handling on database inserts.

**Symptoms:**
- Address creation appears successful
- User gets confirmation (Slack "wahoo!" or dashboard success)
- But address is NOT in database
- Emails bounce with "550 5.0.1 Recipient rejected"

**Root Cause:** `db.DB.Create()` had no error checking - if database insert failed, code continued as if it succeeded.

## What Was Fixed

### 1. Slack Bot Address Creation (Commit 6a35784)
**File:** `pkg/slackevents/slackevents.go` line 183
**Before:** `db.DB.Create(&email)` - no error check
**After:** Check `result.Error`, log success/failure, show error to user if failed

### 2. Dashboard Address Creation (Commit b11f7cb) - **THIS IS NEW**
**File:** `pkg/slackevents/slackevents.go` line 357
**Before:** `db.DB.Create(&address)` - no error check
**After:** Check `result.Error`, log success/failure, return 500 if failed

## Deployment Steps

### Step 1: Pull Latest Code

```bash
cd /path/to/temp-email
git pull origin master
```

You should see:
```
Updating 047e8dc..b11f7cb
Fast-forward
 check-db-connection.sh       | 108 +++++++++++++++++++++++++++++++++++++++++++
 pkg/slackevents/slackevents.go |  28 +++++++++--
 2 files changed, 133 insertions(+), 3 deletions(-)
```

### Step 2: Rebuild Containers

```bash
docker-compose down
docker-compose up -d --build
```

Wait for containers to start:
```bash
docker-compose ps
```

All services should show "Up":
```
NAME                COMMAND              STATUS
temp-email-db-1     postgres             Up
temp-email-web-1    /app/main            Up
```

### Step 3: Check Database Connection

Run the diagnostic script:
```bash
./check-db-connection.sh
```

**Expected Output:**
- ✅ Database connection: OK
- ✅ Tables exist (addresses, emails)
- ✅ Container status: Up
- ✅ No errors in logs

**If you see errors:**
- ❌ Database connection failed → `docker-compose restart db`
- ❌ Tables missing → Check if migrations ran
- ❌ Container not running → Check `docker-compose logs db`

### Step 4: Test Address Creation

#### Test A: Dashboard Creation

1. Go to `https://mail.rushilchopra.com/dashboard`
2. Log in with your password
3. Create a new address (e.g., name: "test", duration: 24 hours)
4. **If successful:** Address appears in list with green "Active" badge
5. **If failed:** Dashboard shows error message in console or network tab

#### Test B: Slack Bot Creation

1. Go to your Slack channel
2. Type: `gib email`
3. **If successful:** Bot replies "wahoo! your temporary 24-hour email address is..."
4. **If failed:** Bot replies "uh oh! something went wrong creating that address..."

### Step 5: Verify Address in Database

After creating an address (let's say `abc123`):

```bash
./test-address-creation.sh abc123
```

**Expected Output:**
```
✅ Address found
SUCCESS: Created address abc123 for user ...
```

**If address NOT found:**
```
❌ Address not found
ERROR: Failed to create address abc123: [error message]
```

The error message will tell you exactly what's wrong (connection timeout, permission denied, etc.)

### Step 6: Test Email Delivery

Send a test email to your created address:

```bash
echo "Test email body" | mail -s "Test Subject" abc123@rushilchopra.com
```

Check SMTP logs:
```bash
docker-compose logs smtp --tail=20
```

**Expected Output:**
```
ACCEPT: Email received for abc123@rushilchopra.com from sender@example.com
```

**If rejected:**
```
REJECT: Address not found or expired: abc123 (from: sender@example.com)
```

This means the address is still not in the database.

### Step 7: Monitor Logs

Watch logs in real-time while testing:

```bash
# Terminal 1: Web server logs
docker-compose logs -f web

# Terminal 2: SMTP server logs  
docker-compose logs -f smtp

# Terminal 3: Database logs
docker-compose logs -f db
```

**What to look for:**

**SUCCESS logs:**
```
SUCCESS: Created address abc123 for user U12345 (expires: 2024-01-15T12:00:00Z)
SUCCESS: Created address xyz789 via dashboard (expires: 2024-01-15T12:00:00Z)
```

**ERROR logs:**
```
ERROR: Failed to create address abc123 for user U12345: connection timeout
ERROR: Failed to create address xyz789 via dashboard: duplicate key
```

**ACCEPT/REJECT logs:**
```
ACCEPT: Email received for abc123@rushilchopra.com from sender@example.com
REJECT: Address not found or expired: xyz789 (from: test@gmail.com)
```

## Troubleshooting

### Issue 1: All Addresses Still Being Rejected

**Diagnosis:**
```bash
docker-compose logs web | grep -E "SUCCESS|ERROR"
```

**If you see ERROR logs:**
- Read the error message carefully
- Common causes: database connection lost, permission denied, constraint violation

**If you see NO logs at all:**
- Code might not be deployed yet
- Check: `docker-compose exec web cat /app/main | md5sum` to verify binary changed
- Try: `docker-compose up -d --build --force-recreate`

**If you see SUCCESS logs but emails still rejected:**
- Address might be in database but expired
- Check: `./test-address-creation.sh [address-id]`
- Look for `expires_at` and `hours_remaining`

### Issue 2: Database Connection Failed

**Diagnosis:**
```bash
docker-compose ps
docker-compose logs db --tail=50
```

**Solutions:**
```bash
# Restart database
docker-compose restart db

# If that doesn't work, recreate it
docker-compose down
docker-compose up -d
```

### Issue 3: Addresses Work Sometimes, Fail Other Times

**Likely Cause:** Database connection is unstable (timeouts)

**Check connection pool:**
```bash
docker-compose logs web | grep -i "connection"
```

**Solution:** Increase connection timeout or pool size in database config

### Issue 4: Dashboard Shows Success But Address Not in DB

**This means the fix isn't deployed yet!**

**Verify fix is deployed:**
```bash
cd /path/to/temp-email
git log --oneline -5
```

Should show:
```
b11f7cb fix: add error handling for dashboard address creation
6a35784 fix: add error handling for database operations and comprehensive logging
```

**If not:** Run `git pull origin master` again

## Verification Checklist

After deployment, verify everything works:

- [ ] Database connection test passes (`./check-db-connection.sh`)
- [ ] Create address via Slack → Shows "wahoo!" → Address appears in database
- [ ] Create address via dashboard → Shows success → Address appears in database
- [ ] Send email to created address → Email is accepted and appears in Slack/dashboard
- [ ] Check logs show SUCCESS messages for created addresses
- [ ] Check logs show ACCEPT messages for received emails
- [ ] No ERROR or REJECT logs for valid addresses
- [ ] Old addresses (created before fix) still work
- [ ] Address expiration still works (expired addresses are rejected)

## Expected Behavior After Fix

### Successful Address Creation (Slack):
1. User types "gib email"
2. Database insert succeeds
3. Log: `SUCCESS: Created address abc123...`
4. Bot replies: "wahoo! your temporary 24-hour email address is abc123@rushilchopra.com"
5. Slack notification sent (if dashboard)
6. Email sent to address → ACCEPT → Posted to Slack

### Failed Address Creation (Slack):
1. User types "gib email"
2. Database insert fails
3. Log: `ERROR: Failed to create address abc123: connection timeout`
4. Bot replies: "uh oh! something went wrong creating that address..."
5. NO Slack notification
6. User knows to try again

### Successful Address Creation (Dashboard):
1. User submits form
2. Database insert succeeds
3. Log: `SUCCESS: Created address xyz789 via dashboard...`
4. Dashboard shows new address in list
5. Slack notification sent
6. Email sent to address → ACCEPT → Appears in dashboard

### Failed Address Creation (Dashboard):
1. User submits form
2. Database insert fails
3. Log: `ERROR: Failed to create address xyz789 via dashboard: ...`
4. Dashboard shows error (check browser console)
5. NO Slack notification
6. User sees error and can try again

## Monitoring

After deployment, monitor for 24 hours:

```bash
# Check for any errors
docker-compose logs web | grep ERROR

# Check success rate
docker-compose logs web | grep "SUCCESS: Created" | wc -l

# Check rejection rate
docker-compose logs smtp | grep REJECT | wc -l

# Check acceptance rate
docker-compose logs smtp | grep ACCEPT | wc -l
```

**Healthy system:**
- SUCCESS logs > 0
- ACCEPT logs > 0
- ERROR logs = 0 (or very few)
- REJECT logs only for expired/deleted addresses

## Support

If issues persist after deployment:

1. **Run diagnostics:**
   ```bash
   ./check-db-connection.sh > diagnostic.txt
   ./test-address-creation.sh [failing-address] >> diagnostic.txt
   ```

2. **Collect logs:**
   ```bash
   docker-compose logs web --tail=100 > web.log
   docker-compose logs smtp --tail=100 > smtp.log
   docker-compose logs db --tail=100 > db.log
   ```

3. **Check the error messages in logs** - they'll tell you exactly what's wrong

4. **Common error messages and meanings:**
   - "connection timeout" → Database is slow/unreachable
   - "duplicate key" → Address ID collision (very rare)
   - "permission denied" → Database user lacks INSERT permission
   - "relation does not exist" → Tables not created

---

**Status:** Fix deployed in commits `6a35784` and `b11f7cb`
**Next Action:** Deploy to production and test address creation
**Expected Result:** All address creation succeeds OR shows clear error message
