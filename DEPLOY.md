# Deployment Guide 🚀

Quick guide to deploy the simple password authentication.

## Step 1: Update Code on Server

SSH into your server and pull the latest changes:

```bash
ssh ubuntu@your-server-ip
cd temp-email-service
git pull origin master
```

## Step 2: Set Dashboard Password

Edit your `.env` file to add your password:

```bash
nano .env
```

Add this line (replace with your own secure password):
```bash
DASHBOARD_PASSWORD=your_secure_password_here
```

**Important**: Choose a strong password! This protects your dashboard from abuse.

If you don't set this, it defaults to `admin` (not recommended for production).

Save and exit (Ctrl+X, Y, Enter)

## Step 3: Rebuild and Restart

Rebuild the Docker containers with the new code:

```bash
docker-compose down
docker-compose up -d --build
```

## Step 4: Test Authentication

1. Visit your dashboard: `https://mail.rushilchopra.com/dashboard`
2. You should be redirected to the login page
3. Enter your password from the `.env` file
4. You should be redirected to the dashboard!

## Step 5: Logout Test

Click the logout button (if you add one) or clear cookies to test re-authentication.

## How It Works

### Simple Password Auth
- No OAuth providers required
- Password stored in environment variable
- Session managed with secure cookies
- Sessions last 7 days
- Clean login page with error handling

### Security Features
- Password required for all dashboard and API routes
- Secure HTTP-only cookies
- Session-based authentication
- No passwords stored in database
- Protection against unauthorized address creation

## Managing Sessions

Sessions are stored in memory (map). If you restart the service, all users will need to login again.

For production, consider:
- Redis for session storage (persistent across restarts)
- Rate limiting on login endpoint
- HTTPS only (already configured with Cloudflare)

## Removing Authentication (Not Recommended)

If you want to disable authentication temporarily for testing:

```bash
# In .env, add:
SKIP_AUTH=true
```

Then restart:
```bash
docker-compose restart web
```

**Warning**: This makes your dashboard publicly accessible!

## Environment Variables

```bash
# Required
DASHBOARD_PASSWORD=your_password

# Optional (for development only)
SKIP_AUTH=true  # Bypasses authentication - DO NOT USE IN PRODUCTION
```

## Troubleshooting

### "Incorrect password" error
- Check your `.env` file has `DASHBOARD_PASSWORD=your_password`
- Ensure you restarted after editing .env: `docker-compose restart web`
- Password is case-sensitive

### Still seeing login.new references
- Make sure you pulled latest code: `git pull origin master`
- Rebuild containers: `docker-compose up -d --build`

### Logged out automatically
- Sessions expire after 7 days
- Service restarts clear all sessions (in-memory storage)
- Clear your cookies if having issues

### Can't access dashboard
- Check service is running: `docker-compose ps`
- Check logs: `docker-compose logs web`
- Verify port 3001 is accessible

## Next Steps

Your dashboard is now protected with simple password authentication! 

Users can:
- ✅ View all temporary addresses
- ✅ Create new addresses with custom names and durations  
- ✅ See usage statistics
- ✅ Delete addresses
- ✅ Beautiful modern UI

All protected by your password! 🔐
