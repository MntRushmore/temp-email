package slackevents

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/DusanKasan/parsemail"
	"github.com/cjdenio/temp-email/pkg/db"
	"github.com/cjdenio/temp-email/pkg/util"
	"github.com/gin-gonic/gin"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"gorm.io/gorm"
)

var Client *slack.Client

func topLevelMessage(ev *slackevents.MessageEvent) bool {
	return ev.Channel == os.Getenv("SLACK_CHANNEL") && ev.ThreadTimeStamp == ""
}

func Start() {
	Client = slack.New(os.Getenv("SLACK_TOKEN"))

	r := gin.Default()

	r.POST("/slack/events", func(c *gin.Context) {
		body, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			c.Writer.WriteHeader(http.StatusBadRequest)
			return
		}
		sv, err := slack.NewSecretsVerifier(c.Request.Header, os.Getenv("SLACK_SIGNING_SECRET"))
		if err != nil {
			c.Writer.WriteHeader(http.StatusBadRequest)
			return
		}
		if _, err := sv.Write(body); err != nil {
			c.Writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		if err := sv.Ensure(); err != nil {
			c.Writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
		if err != nil {
			c.Writer.WriteHeader(http.StatusInternalServerError)
			return
		}

		if eventsAPIEvent.Type == slackevents.URLVerification {
			var r *slackevents.ChallengeResponse
			err := json.Unmarshal([]byte(body), &r)
			if err != nil {
				c.Writer.WriteHeader(http.StatusInternalServerError)
				return
			}
			c.Writer.Header().Set("Content-Type", "text")
			c.Writer.Write([]byte(r.Challenge))
		}
		if eventsAPIEvent.Type == slackevents.CallbackEvent {
			innerEvent := eventsAPIEvent.InnerEvent
			switch ev := innerEvent.Data.(type) {
			case *slackevents.MessageEvent:
				// Feature 1: Stats command
				if ev.SubType == "" && topLevelMessage(ev) && strings.Contains(strings.ToLower(ev.Text), "email stats") {
					var totalCount int64
					db.DB.Model(&db.Address{}).Count(&totalCount)
					
					var activeCount int64
					db.DB.Model(&db.Address{}).Where("expires_at > NOW()").Count(&activeCount)
					
					var emailCount int64
					db.DB.Model(&db.Email{}).Count(&emailCount)
					
					Client.PostMessage(ev.Channel, 
						slack.MsgOptionText(fmt.Sprintf("📊 *Email Stats*\n\n📬 Total addresses created: %d\n✅ Currently active: %d\n📨 Total emails received: %d", totalCount, activeCount, emailCount), false),
						slack.MsgOptionTS(ev.TimeStamp),
					)
				} else if ev.SubType == "" && topLevelMessage(ev) && strings.Contains(strings.ToLower(ev.Text), "gib email") {
					// Parse custom duration and name from message
					text := strings.ToLower(ev.Text)
					parts := strings.Fields(text)
					
					// Default values
					duration := 24 * time.Hour
					prefix := ""
					
					// Feature 2: Custom duration - check for time specs
					for _, part := range parts {
						if strings.HasSuffix(part, "h") {
							if hours := strings.TrimSuffix(part, "h"); hours != "" {
								if h, err := time.ParseDuration(hours + "h"); err == nil {
									duration = h
								}
							}
						} else if strings.HasSuffix(part, "d") {
							if days := strings.TrimSuffix(part, "d"); days != "" {
								if d, err := time.ParseDuration(days + "h"); err == nil {
									duration = d * 24
								}
							}
						}
					}
					
					// Feature 3: Named addresses - check for custom name
					for i, part := range parts {
						if part == "email" && i+1 < len(parts) {
							nextPart := parts[i+1]
							// Only use as name if it's not a duration specifier
							if !strings.HasSuffix(nextPart, "h") && !strings.HasSuffix(nextPart, "d") {
								prefix = nextPart + "-"
							}
							break
						}
					}
					
					address := prefix + util.GenerateEmailAddress()

					err = Client.AddReaction("thumb", slack.ItemRef{
						Channel:   ev.Channel,
						Timestamp: ev.TimeStamp,
					})
					if err != nil {
						fmt.Println(err)
					}
					
					// Custom message based on duration
					durationHours := int(duration.Hours())
					durationText := fmt.Sprintf("%d-hour", durationHours)
					if durationHours >= 24 {
						durationDays := durationHours / 24
						if durationDays == 1 {
							durationText = "24-hour"
						} else {
							durationText = fmt.Sprintf("%d-day", durationDays)
						}
					}

					Client.PostMessage(
						ev.Channel,
						slack.MsgOptionText(fmt.Sprintf(`wahoo! your temporary %s email address is %s@%s
						
to stop receiving emails, delete your 'gib email' message.

i'll post emails in this thread :arrow_down:`, durationText, address, os.Getenv("DOMAIN")), false),
						slack.MsgOptionTS(ev.TimeStamp),
					)

					email := db.Address{
						ID:        address,
						CreatedAt: time.Now(),
						ExpiresAt: time.Now().Add(duration),
						Timestamp: ev.TimeStamp,
						User:      ev.User,
					}

					db.DB.Create(&email)
				} else if ev.SubType == "" && topLevelMessage(ev) && strings.HasPrefix(strings.ToLower(ev.Text), "gib ") {
					Client.PostMessage(ev.Channel, slack.MsgOptionText(fmt.Sprintf("unfortunately i am unable to _\"gib %s\"_. maybe try _\"gib email\"_?", strings.TrimPrefix(strings.ToLower(ev.Text), "gib ")), false), slack.MsgOptionTS(ev.TimeStamp))
				} else if (ev.SubType == "message_deleted" || (ev.SubType == "message_changed" && ev.Message.SubType == "tombstone")) && topLevelMessage(ev) {
					var address db.Address
					tx := db.DB.Where("timestamp = ? AND expires_at > NOW()", ev.PreviousMessage.TimeStamp).First(&address)

					if tx.Error == nil {
						address.ExpiresAt = time.Now()
						address.ExpiredMessageSent = true
						tx = db.DB.Save(&address)
						if tx.Error == nil {
							Client.PostMessage(
								os.Getenv("SLACK_CHANNEL"),
								slack.MsgOptionText(":x: since you deleted your message, this address has been deactivated.", false),
								slack.MsgOptionTS(address.Timestamp),
							)
						}

					}
				}
			}
		}
	})

	r.POST("/slack/interactivity", func(c *gin.Context) {
		body, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			c.Writer.WriteHeader(http.StatusBadRequest)
			return
		}
		sv, err := slack.NewSecretsVerifier(c.Request.Header, os.Getenv("SLACK_SIGNING_SECRET"))
		if err != nil {
			c.Writer.WriteHeader(http.StatusBadRequest)
			return
		}
		if _, err := sv.Write(body); err != nil {
			c.Writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		if err := sv.Ensure(); err != nil {
			c.Writer.WriteHeader(http.StatusUnauthorized)
			return
		}

		form, err := url.ParseQuery(string(body))
		if err != nil {
			fmt.Println(err)
		}

		var payload slack.InteractionCallback

		err = json.Unmarshal([]byte(form.Get("payload")), &payload)
		if err != nil {
			fmt.Printf("Could not parse action response JSON: %v", err)
		}

		if payload.ActionCallback.BlockActions[0].ActionID == "reactivate" {
			id := payload.ActionCallback.BlockActions[0].Value
			var address db.Address
			tx := db.DB.Where("id = ? AND expires_at < NOW()", id).First(&address)
			if tx.Error != nil {
				return
			}

			if payload.User.ID != address.User {
				Client.PostEphemeral(os.Getenv("SLACK_CHANNEL"), payload.User.ID, slack.MsgOptionTS(address.Timestamp), slack.MsgOptionText("whatcha tryin' to pull here :face_with_raised_eyebrow:", false))
				return
			}

			address.ExpiresAt = time.Now().Add(24 * time.Hour)
			address.ExpiredMessageSent = false

			db.DB.Save(&address)

			Client.PostMessage(os.Getenv("SLACK_CHANNEL"), slack.MsgOptionTS(address.Timestamp), slack.MsgOptionText("This address will be available for another 24 hours!", false))
			Client.RemoveReaction("clock1", slack.ItemRef{
				Channel:   os.Getenv("SLACK_CHANNEL"),
				Timestamp: address.Timestamp,
			})
		}
	})

	// Dashboard routes
	r.GET("/dashboard", func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(200, getDashboardHTML())
	})

	r.GET("/api/addresses", func(c *gin.Context) {
		var addresses []db.Address
		db.DB.Order("created_at DESC").Find(&addresses)
		c.JSON(200, addresses)
	})

	r.GET("/api/emails/:addressId", func(c *gin.Context) {
		var emails []db.Email
		db.DB.Where("address_id = ?", c.Param("addressId")).Order("id DESC").Find(&emails)
		c.JSON(200, emails)
	})

	r.POST("/api/addresses", func(c *gin.Context) {
		var req struct {
			Name     string `json:"name"`
			Duration int    `json:"duration"` // in hours
		}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request"})
			return
		}

		prefix := ""
		if req.Name != "" {
			prefix = req.Name + "-"
		}
		duration := 24
		if req.Duration > 0 {
			duration = req.Duration
		}

		address := db.Address{
			ID:        prefix + util.GenerateEmailAddress(),
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(time.Duration(duration) * time.Hour),
			Timestamp: "",
			User:      "dashboard",
		}

		db.DB.Create(&address)
		c.JSON(200, address)
	})

	r.DELETE("/api/addresses/:id", func(c *gin.Context) {
		var address db.Address
		if err := db.DB.Where("id = ?", c.Param("id")).First(&address).Error; err != nil {
			c.JSON(404, gin.H{"error": "Address not found"})
			return
		}

		address.ExpiresAt = time.Now()
		db.DB.Save(&address)
		c.JSON(200, gin.H{"success": true})
	})

	r.GET("/:email", func(c *gin.Context) {
		var rawEmail db.Email
		tx := db.DB.Where("id = ?", c.Param("email")).First(&rawEmail)
		if tx.Error == gorm.ErrRecordNotFound {
			c.String(404, "404 email not found :(")
			return
		} else if tx.Error != nil {
			c.String(500, "aaaaaaaaaaaaaaaaaaaa something went wrong")
			return
		}

		email, err := parsemail.Parse(strings.NewReader(rawEmail.Content))
		if err != nil {
			c.String(500, "aaaaaaaaaaaaaaaaaaaa something went wrong")
			return
		}

		if email.HTMLBody != "" {
			c.Header("Content-Type", "text/html; charset=utf-8")

			c.String(200, email.HTMLBody)
		} else if email.TextBody != "" {

			c.Header("Content-Type", "text/plain; charset=utf-8")

			c.String(200, email.TextBody)
		} else {
			c.Header("Content-Type", "text/plain")

			c.String(200, "Something went wrong: this message has no content :(")
		}
	})

	log.Println("Starting up HTTP server...")

	r.Run(":3001")
}

func getDashboardHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>RMail Dashboard</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        :root {
            --bg: #0f0f1e;
            --bg-secondary: #1a1a2e;
            --surface: #16213e;
            --surface-hover: #1e2d50;
            --primary: #6366f1;
            --primary-hover: #4f46e5;
            --text: #e2e8f0;
            --text-secondary: #94a3b8;
            --success: #10b981;
            --danger: #ef4444;
            --border: rgba(148, 163, 184, 0.1);
            --shadow: rgba(0, 0, 0, 0.3);
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Inter', sans-serif;
            background: var(--bg);
            color: var(--text);
            line-height: 1.6;
            min-height: 100vh;
        }

        .container {
            max-width: 1400px;
            margin: 0 auto;
            padding: 2rem;
        }

        header {
            margin-bottom: 3rem;
            animation: fadeIn 0.6s ease-out;
        }

        h1 {
            font-size: 2.5rem;
            font-weight: 700;
            background: linear-gradient(135deg, #6366f1 0%, #a855f7 100%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
            margin-bottom: 0.5rem;
        }

        .subtitle {
            color: var(--text-secondary);
            font-size: 1.1rem;
        }

        .stats {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1.5rem;
            margin-bottom: 3rem;
            animation: fadeIn 0.6s ease-out 0.1s both;
        }

        .stat-card {
            background: linear-gradient(135deg, var(--surface) 0%, var(--bg-secondary) 100%);
            padding: 1.5rem;
            border-radius: 16px;
            border: 1px solid var(--border);
            backdrop-filter: blur(10px);
            transition: all 0.3s ease;
        }

        .stat-card:hover {
            transform: translateY(-4px);
            border-color: var(--primary);
            box-shadow: 0 8px 32px rgba(99, 102, 241, 0.15);
        }

        .stat-label {
            color: var(--text-secondary);
            font-size: 0.875rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            margin-bottom: 0.5rem;
        }

        .stat-value {
            font-size: 2.5rem;
            font-weight: 700;
            background: linear-gradient(135deg, var(--primary) 0%, #a855f7 100%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
        }

        .create-section {
            background: var(--surface);
            padding: 2rem;
            border-radius: 16px;
            border: 1px solid var(--border);
            margin-bottom: 3rem;
            animation: fadeIn 0.6s ease-out 0.2s both;
        }

        .create-form {
            display: grid;
            grid-template-columns: 2fr 1fr auto;
            gap: 1rem;
            align-items: end;
        }

        @media (max-width: 768px) {
            .create-form {
                grid-template-columns: 1fr;
            }
        }

        .form-group {
            display: flex;
            flex-direction: column;
        }

        label {
            color: var(--text-secondary);
            font-size: 0.875rem;
            margin-bottom: 0.5rem;
            font-weight: 500;
        }

        input, select {
            background: var(--bg-secondary);
            border: 1px solid var(--border);
            border-radius: 10px;
            padding: 0.875rem 1rem;
            color: var(--text);
            font-size: 1rem;
            transition: all 0.2s;
        }

        input:focus, select:focus {
            outline: none;
            border-color: var(--primary);
            box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.1);
        }

        button {
            background: linear-gradient(135deg, var(--primary) 0%, #4f46e5 100%);
            color: white;
            border: none;
            padding: 0.875rem 2rem;
            border-radius: 10px;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.2s;
            font-size: 1rem;
            white-space: nowrap;
        }

        button:hover {
            transform: translateY(-2px);
            box-shadow: 0 8px 24px rgba(99, 102, 241, 0.3);
        }

        button:active {
            transform: translateY(0);
        }

        .addresses-section {
            animation: fadeIn 0.6s ease-out 0.3s both;
        }

        .section-title {
            font-size: 1.5rem;
            font-weight: 700;
            margin-bottom: 1.5rem;
            color: var(--text);
        }

        .addresses-grid {
            display: grid;
            gap: 1rem;
        }

        .address-card {
            background: var(--surface);
            border: 1px solid var(--border);
            border-radius: 12px;
            padding: 1.5rem;
            transition: all 0.3s ease;
            animation: slideUp 0.4s ease-out backwards;
        }

        .address-card:hover {
            border-color: var(--primary);
            transform: translateX(4px);
        }

        .address-header {
            display: flex;
            justify-content: space-between;
            align-items: start;
            margin-bottom: 1rem;
        }

        .address-email {
            font-size: 1.25rem;
            font-weight: 600;
            color: var(--primary);
            word-break: break-all;
        }

        .address-actions {
            display: flex;
            gap: 0.5rem;
        }

        .btn-icon {
            background: var(--bg-secondary);
            border: 1px solid var(--border);
            padding: 0.5rem;
            border-radius: 8px;
            cursor: pointer;
            transition: all 0.2s;
        }

        .btn-icon:hover {
            background: var(--danger);
            border-color: var(--danger);
        }

        .address-meta {
            display: flex;
            gap: 2rem;
            color: var(--text-secondary);
            font-size: 0.875rem;
            flex-wrap: wrap;
        }

        .meta-item {
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }

        .badge {
            display: inline-block;
            padding: 0.25rem 0.75rem;
            border-radius: 20px;
            font-size: 0.75rem;
            font-weight: 600;
            text-transform: uppercase;
        }

        .badge-active {
            background: rgba(16, 185, 129, 0.2);
            color: var(--success);
        }

        .badge-expired {
            background: rgba(239, 68, 68, 0.2);
            color: var(--danger);
        }

        .loading {
            text-align: center;
            padding: 3rem;
            color: var(--text-secondary);
        }

        .spinner {
            border: 3px solid var(--border);
            border-top: 3px solid var(--primary);
            border-radius: 50%;
            width: 40px;
            height: 40px;
            animation: spin 1s linear infinite;
            margin: 0 auto 1rem;
        }

        @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
        }

        @keyframes fadeIn {
            from {
                opacity: 0;
                transform: translateY(10px);
            }
            to {
                opacity: 1;
                transform: translateY(0);
            }
        }

        @keyframes slideUp {
            from {
                opacity: 0;
                transform: translateY(20px);
            }
            to {
                opacity: 1;
                transform: translateY(0);
            }
        }

        .empty-state {
            text-align: center;
            padding: 4rem 2rem;
            color: var(--text-secondary);
        }

        .empty-icon {
            font-size: 4rem;
            margin-bottom: 1rem;
            opacity: 0.3;
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>📧 RMail Dashboard</h1>
            <p class="subtitle">Manage your temporary email addresses</p>
        </header>

        <div class="stats" id="stats">
            <div class="stat-card">
                <div class="stat-label">Total Addresses</div>
                <div class="stat-value" id="totalAddresses">-</div>
            </div>
            <div class="stat-card">
                <div class="stat-label">Active Now</div>
                <div class="stat-value" id="activeAddresses">-</div>
            </div>
            <div class="stat-card">
                <div class="stat-label">Emails Received</div>
                <div class="stat-value" id="totalEmails">-</div>
            </div>
        </div>

        <div class="create-section">
            <form class="create-form" id="createForm">
                <div class="form-group">
                    <label for="name">Custom Name (optional)</label>
                    <input type="text" id="name" placeholder="e.g., github, testing, work">
                </div>
                <div class="form-group">
                    <label for="duration">Duration</label>
                    <select id="duration">
                        <option value="24">24 hours</option>
                        <option value="48">48 hours</option>
                        <option value="72">3 days</option>
                        <option value="168">7 days</option>
                    </select>
                </div>
                <button type="submit">✨ Create Address</button>
            </form>
        </div>

        <div class="addresses-section">
            <h2 class="section-title">Your Addresses</h2>
            <div id="addressesList">
                <div class="loading">
                    <div class="spinner"></div>
                    <p>Loading addresses...</p>
                </div>
            </div>
        </div>
    </div>

    <script>
        const API_BASE = '';

        async function loadStats() {
            const res = await fetch(API_BASE + '/api/addresses');
            const addresses = await res.json();
            
            const now = new Date();
            const active = addresses.filter(a => new Date(a.ExpiresAt) > now).length;
            
            document.getElementById('totalAddresses').textContent = addresses.length;
            document.getElementById('activeAddresses').textContent = active;
            
            // Load email count (simplified - just count emails in UI)
            let totalEmails = 0;
            for (const addr of addresses) {
                const emailRes = await fetch(API_BASE + '/api/emails/' + addr.ID);
                const emails = await emailRes.json();
                totalEmails += emails.length;
            }
            document.getElementById('totalEmails').textContent = totalEmails;
        }

        async function loadAddresses() {
            const res = await fetch(API_BASE + '/api/addresses');
            const addresses = await res.json();
            
            const container = document.getElementById('addressesList');
            
            if (addresses.length === 0) {
                container.innerHTML = '<div class="empty-state"><div class="empty-icon">📭</div><p>No addresses yet. Create your first one above!</p></div>';
                return;
            }
            
            const now = new Date();
            container.innerHTML = addresses.map((addr, i) => {
                const isActive = new Date(addr.ExpiresAt) > now;
                const created = new Date(addr.CreatedAt).toLocaleString();
                const expires = new Date(addr.ExpiresAt).toLocaleString();
                
                return '<div class="address-card" style="animation-delay: ' + (i * 0.05) + 's">' +
                    '<div class="address-header">' +
                        '<div class="address-email">' + addr.ID + '@' + window.location.hostname.replace('mail.', '') + '</div>' +
                        '<div class="address-actions">' +
                            '<button class="btn-icon" onclick="deleteAddress(\'' + addr.ID + '\')" title="Delete">🗑️</button>' +
                        '</div>' +
                    '</div>' +
                    '<div class="address-meta">' +
                        '<div class="meta-item">' +
                            '<span class="badge ' + (isActive ? 'badge-active' : 'badge-expired') + '">' + (isActive ? 'Active' : 'Expired') + '</span>' +
                        '</div>' +
                        '<div class="meta-item">📅 Created: ' + created + '</div>' +
                        '<div class="meta-item">⏰ Expires: ' + expires + '</div>' +
                    '</div>' +
                '</div>';
            }).join('');
        }

        document.getElementById('createForm').addEventListener('submit', async (e) => {
            e.preventDefault();
            
            const name = document.getElementById('name').value;
            const duration = parseInt(document.getElementById('duration').value);
            
            const res = await fetch(API_BASE + '/api/addresses', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name, duration })
            });
            
            if (res.ok) {
                document.getElementById('name').value = '';
                await loadAddresses();
                await loadStats();
            }
        });

        async function deleteAddress(id) {
            if (!confirm('Delete this address?')) return;
            
            await fetch(API_BASE + '/api/addresses/' + id, {
                method: 'DELETE'
            });
            
            await loadAddresses();
            await loadStats();
        }

        // Load on page load
        loadStats();
        loadAddresses();
        
        // Auto-refresh every 30 seconds
        setInterval(() => {
            loadAddresses();
            loadStats();
        }, 30000);
    </script>
</body>
</html>`
}
