# ⚡ Quick Deploy - Password Authentication

**Super fast deployment guide for the new simple password auth!**

## 🚀 One-Command Deployment

SSH into your server and run:

```bash
cd temp-email-service
curl -s https://raw.githubusercontent.com/MntRushmore/temp-email/master/update-auth.sh | bash -s "your_password_here"
```

Replace `your_password_here` with your actual password.

**That's it!** 🎉

## 📋 Manual Deployment (5 Steps)

If you prefer manual steps:

```bash
# 1. SSH into server
ssh ubuntu@your-server-ip

# 2. Navigate to project
cd temp-email-service

# 3. Pull latest code
git pull origin master

# 4. Update .env file
nano .env
# Add this line:
# DASHBOARD_PASSWORD=your_password_here
# Save with Ctrl+X, Y, Enter

# 5. Rebuild and restart
docker-compose down
docker-compose up -d --build
```

## ✅ Verify Deployment

1. Open browser: `https://mail.rushilchopra.com/dashboard`
2. You should see a clean login page
3. Enter your password
4. Access the dashboard! 🎨

## 🎯 What You Get

### Login Page
- Clean, modern design
- Password input field
- Error handling for wrong passwords
- Mobile responsive

### Protected Routes
- `/dashboard` - Main interface
- `/api/addresses` - Address management
- `/api/emails/:id` - Email viewing

### Features
- ✅ View all email addresses
- ✅ Create addresses with custom names
- ✅ Set custom expiration times
- ✅ View usage statistics
- ✅ Delete addresses
- ✅ Beautiful modern UI

## 🔧 Configuration

Only one environment variable needed:

```bash
DASHBOARD_PASSWORD=your_secure_password
```

**Security Tips:**
- Use a strong password (mix of letters, numbers, symbols)
- Don't share the password publicly
- Change password regularly
- Don't use default "admin" password

## 🔄 Change Password Later

Just run the update script again with a new password:

```bash
cd temp-email-service
./update-auth.sh new_password
```

Or manually edit `.env` and restart:

```bash
nano .env
# Update DASHBOARD_PASSWORD=new_password
docker-compose restart web
```

## ❓ Troubleshooting

### "Incorrect password" error
```bash
# Check your .env file
cat .env | grep DASHBOARD_PASSWORD

# Make sure it matches what you're typing
# Passwords are case-sensitive!
```

### Still seeing old login page
```bash
# Clear browser cache and hard reload
# Chrome/Firefox: Ctrl+Shift+R
# Safari: Cmd+Shift+R

# Or rebuild containers
docker-compose up -d --build
```

### Services not starting
```bash
# Check logs
docker-compose logs web

# Restart everything
docker-compose down
docker-compose up -d --build
```

### Port conflicts
```bash
# Check what's using the ports
sudo netstat -tlnp | grep 3001

# Stop conflicting services or change ports
```

## 📊 Monitoring

Check service health:

```bash
# View all services
docker-compose ps

# View logs
docker-compose logs -f web

# Check specific service
docker-compose logs web --tail=50
```

## 🛡️ Security Notes

### Current Security Features
- ✅ Password-based authentication
- ✅ Session cookies (7-day expiration)
- ✅ HTTP-only cookies (prevent XSS)
- ✅ Secure cookies (HTTPS only)
- ✅ Protected API routes

### Recommended (Future)
- Rate limiting on login endpoint
- Captcha for brute force protection
- Password hashing (if storing users)
- 2FA for additional security
- Audit logs for login attempts

**For now, the simple password is perfect for your use case!** 🔐

## 📱 Mobile Access

The dashboard is fully responsive and works great on:
- 📱 iPhone/iPad
- 🤖 Android phones/tablets
- 💻 Desktop browsers
- 🖥️ Tablets

## 🎨 UI Features

### Dashboard
- Clean, modern gradient design
- Smooth animations
- Real-time updates
- Mobile-optimized

### Stats Cards
- Total addresses created
- Currently active addresses
- Total emails received
- Hover effects and gradients

### Address Management
- Create with custom names
- Set expiration (24h, 48h, 3d, 7d)
- Delete addresses instantly
- View creation/expiration times

## 🔗 Useful Links

- **Dashboard**: https://mail.rushilchopra.com/dashboard
- **Login Page**: https://mail.rushilchopra.com/login
- **Logout**: https://mail.rushilchopra.com/logout

## 📚 Documentation

- [AUTH_CHANGES.md](AUTH_CHANGES.md) - Detailed changes explanation
- [DEPLOY.md](DEPLOY.md) - Full deployment guide
- [README.md](README.md) - Main project documentation
- [SETUP.md](SETUP.md) - Initial setup guide

## 🆘 Get Help

If something's not working:

1. Check logs: `docker-compose logs web`
2. Verify .env: `cat .env`
3. Check DNS: `./check-dns.sh`
4. Run diagnostics: `./troubleshoot.sh`
5. Rebuild: `docker-compose up -d --build`

## ✨ Next Steps

Your dashboard is now live and protected! 

Try it out:
1. Visit https://mail.rushilchopra.com/dashboard
2. Login with your password
3. Create a temporary email address
4. Send a test email
5. Watch it appear in the dashboard and Slack!

**Enjoy your new secure dashboard!** 🎉
