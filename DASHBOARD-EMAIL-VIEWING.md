# 📧 Dashboard Email Viewing Feature

## Overview

You can now **read emails directly from the dashboard** without needing to sign into Slack!

## Features

### 1. Email Count Badges

Each address card shows a **black badge** with the number of emails received:

```
abc123@rushilchopra.com  [3]
```

The badge appears next to the address name, making it easy to see which addresses have received emails at a glance.

### 2. View Emails Button

Addresses with emails have a **"View X Emails"** button at the bottom of the card:

```
┌────────────────────────────────────────┐
│ abc123@rushilchopra.com [3]         × │
│                                        │
│ [Active] Created Jan 15, 2:30 PM      │
│ Expires Jan 16, 2:30 PM               │
│                                        │
│ [View 3 Emails ▼]                     │
└────────────────────────────────────────┘
```

Click the button to expand and view the email list.

### 3. Email List

When expanded, the card shows all received emails with:
- **Timestamp** - When the email was received
- **"View Full Email" button** - Opens the email in a new tab

```
┌────────────────────────────────────────┐
│ abc123@rushilchopra.com [3]         × │
│                                        │
│ [Active] Created Jan 15, 2:30 PM      │
│ Expires Jan 16, 2:30 PM               │
│                                        │
│ [Hide Emails ▲]                       │
│ ─────────────────────────────────────  │
│ RECEIVED EMAILS                        │
│                                        │
│ ┌────────────────────────────────────┐ │
│ │ Jan 15, 3:00 PM                    │ │
│ │ [View Full Email]                  │ │
│ └────────────────────────────────────┘ │
│                                        │
│ ┌────────────────────────────────────┐ │
│ │ Jan 15, 3:45 PM                    │ │
│ │ [View Full Email]                  │ │
│ └────────────────────────────────────┘ │
│                                        │
│ ┌────────────────────────────────────┐ │
│ │ Jan 15, 4:12 PM                    │ │
│ │ [View Full Email]                  │ │
│ └────────────────────────────────────┘ │
└────────────────────────────────────────┘
```

### 4. Full Email View

Clicking **"View Full Email"** opens the email in a new browser tab, showing:
- **HTML emails** - Full rendered HTML with images, formatting, links
- **Plain text emails** - Clean text display
- **Original formatting** - Exactly as the sender sent it

The URL format is:
```
https://mail.rushilchopra.com/[email-id]
```

This view is **public** (no authentication required), but the email ID is a random string so only people with the link can view it.

## How It Works

### Loading Process

1. **Dashboard loads** → Fetches all addresses
2. **For each address** → Fetches email count from `/api/emails/:addressId`
3. **Shows badges** → Displays count next to address
4. **User clicks button** → Loads full email list
5. **User clicks "View"** → Opens email in new tab via `/:emailId` route

### Auto-Refresh

The dashboard auto-refreshes every **30 seconds**, so new emails will appear automatically without manual refresh.

### Performance

- Email counts are loaded **asynchronously** (non-blocking)
- Email content is only loaded **when you expand** the address
- Full emails open in **new tabs** to keep dashboard responsive

## Use Cases

### 1. Monitor Emails Without Slack

Perfect for:
- Checking emails when Slack is down
- Sharing email access with someone who doesn't have Slack
- Quick monitoring from mobile browser
- Archiving/reviewing old emails

### 2. Multiple Address Management

Easily see which addresses have received emails:
- Email count badges show activity at a glance
- Expand only the addresses you want to check
- No need to scroll through Slack threads

### 3. Email Verification Testing

Great for testing email workflows:
- Create test address
- Trigger email from your app
- View email immediately in dashboard
- Check formatting, links, content

## Design Details

### Visual Hierarchy

- **Email count badge**: Black background, white text, stands out
- **Toggle button**: Light border, changes text on expand/collapse
- **Email items**: Subtle background, hover effect
- **View button**: Outlined style, turns black on hover

### Interaction States

**Collapsed (default):**
```
[View 3 Emails ▼]
```

**Expanded:**
```
[Hide Emails ▲]
```

**Loading:**
```
(Spinner) Loading emails...
```

**No emails:**
```
No emails received yet
```

### Responsive Design

The email viewing feature works on:
- ✅ Desktop browsers
- ✅ Tablets
- ✅ Mobile devices (responsive cards)

## Technical Details

### API Endpoints Used

1. **GET /api/addresses** - List all addresses
2. **GET /api/emails/:addressId** - Get emails for specific address
3. **GET /:emailId** - View full email content

### Data Flow

```
Dashboard → /api/addresses → Address List
           ↓
         /api/emails/:id → Email Count Badges
           ↓
         (User clicks) → Load Email List
           ↓
         (User clicks View) → /:emailId → Full Email
```

### Security

- Dashboard is **password-protected** (DASHBOARD_PASSWORD env var)
- Email viewing requires **valid session cookie**
- Full email view route is **public** but IDs are random/unguessable
- No email content is exposed in address list (only count)

## Screenshots Equivalent

### Before (Address only):
```
┌────────────────────────────────────────┐
│ abc123@rushilchopra.com             × │
│                                        │
│ [Active] Created Jan 15, 2:30 PM      │
│ Expires Jan 16, 2:30 PM               │
└────────────────────────────────────────┘
```

### After (With emails):
```
┌────────────────────────────────────────┐
│ abc123@rushilchopra.com [3]         × │  ← Email count badge
│                                        │
│ [Active] Created Jan 15, 2:30 PM      │
│ Expires Jan 16, 2:30 PM               │
│                                        │
│ [View 3 Emails ▼]                     │  ← New button
└────────────────────────────────────────┘
```

### Expanded:
```
┌────────────────────────────────────────┐
│ abc123@rushilchopra.com [3]         × │
│                                        │
│ [Active] Created Jan 15, 2:30 PM      │
│ Expires Jan 16, 2:30 PM               │
│                                        │
│ [Hide Emails ▲]                       │
│ ─────────────────────────────────────  │  ← Emails section
│ RECEIVED EMAILS                        │
│                                        │
│ (Email items with View buttons)       │
└────────────────────────────────────────┘
```

## Benefits

✅ **No Slack dependency** - View emails without Slack access
✅ **Faster workflow** - All emails in one dashboard
✅ **Better visibility** - See email counts at a glance
✅ **Organized** - Emails grouped by address
✅ **Clean UI** - Matches existing dashboard design
✅ **Auto-refresh** - New emails appear automatically

## Deployment

After pulling the latest code:

```bash
cd /path/to/temp-email
git pull origin master
docker-compose down
docker-compose up -d --build
```

Visit your dashboard at `https://mail.rushilchopra.com/dashboard` and you'll see the new email viewing feature!

## Future Enhancements (Potential)

- 📥 Download email as .eml file
- 🔍 Search emails by sender/subject
- 🗑️ Delete individual emails
- 📎 Show attachment count/list
- 📧 Show email subject in list
- 🏷️ Tag/label emails
- 📊 Email analytics (most active addresses, etc.)

---

**Status:** Deployed in commit `00a51e5`
**Available on:** All addresses with received emails
**No additional setup required**
