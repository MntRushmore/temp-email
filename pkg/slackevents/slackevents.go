package slackevents

import (
	"crypto/rand"
	"encoding/base64"
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
	"github.com/cjdenio/temp-email/pkg/mailgun"
	"github.com/cjdenio/temp-email/pkg/util"
	"github.com/gin-gonic/gin"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"gorm.io/gorm"
)

var Client *slack.Client
var sessions = make(map[string]bool) // Simple session store

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie("auth_token")
		if err != nil || !sessions[token] {
			c.Redirect(302, "/login")
			c.Abort()
			return
		}
		c.Next()
	}
}

func topLevelMessage(ev *slackevents.MessageEvent) bool {
	return ev.Channel == os.Getenv("SLACK_CHANNEL") && ev.ThreadTimeStamp == ""
}

func Start() {
	Client = slack.New(os.Getenv("SLACK_TOKEN"))
	
	// Share Slack client with mailgun package to avoid import cycle
	mailgun.SlackClient = Client

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
						slack.MsgOptionText(fmt.Sprintf("wahoo! your temporary %s email address is %s@%s\n\nto stop receiving emails, delete your 'gib email' message.\n\ni'll post emails in this thread :arrow_down:", durationText, address, os.Getenv("DOMAIN")), false),
						slack.MsgOptionTS(ev.TimeStamp),
					)

					email := db.Address{
						ID:        address,
						CreatedAt: time.Now(),
						ExpiresAt: time.Now().Add(duration),
						Timestamp: ev.TimeStamp,
						User:      ev.User,
					}

					result := db.DB.Create(&email)
					if result.Error != nil {
						log.Printf("ERROR: Failed to create address %s for user %s: %v", address, ev.User, result.Error)
						Client.PostMessage(
							ev.Channel,
							slack.MsgOptionText(fmt.Sprintf("uh oh! something went wrong creating that address. please try again or contact the admin. (error: database insert failed)"), false),
							slack.MsgOptionTS(ev.TimeStamp),
						)
						return
					}
					log.Printf("SUCCESS: Created address %s for user %s (expires: %s)", address, ev.User, email.ExpiresAt.Format(time.RFC3339))
				} else if ev.SubType == "" && topLevelMessage(ev) && strings.HasPrefix(strings.ToLower(ev.Text), "gib ") {
					Client.PostMessage(ev.Channel, slack.MsgOptionText(fmt.Sprintf("unfortunately i am unable to _%s_. maybe try _\"gib email\"_?", strings.ToLower(ev.Text)), false), slack.MsgOptionTS(ev.TimeStamp))
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

			Client.PostMessage(
				os.Getenv("SLACK_CHANNEL"),
				slack.MsgOptionTS(address.Timestamp),
				slack.MsgOptionText("This address will be available for another 24 hours!", false),
			)
			Client.RemoveReaction("clock1", slack.ItemRef{
				Channel:   os.Getenv("SLACK_CHANNEL"),
				Timestamp: address.Timestamp,
			})
		}
	})

	// Login routes
	r.GET("/login", func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(200, getLoginHTML())
	})

	r.POST("/login", func(c *gin.Context) {
		password := c.PostForm("password")
		correctPassword := os.Getenv("DASHBOARD_PASSWORD")
		
		if correctPassword == "" {
			correctPassword = "admin" // Default password if not set
		}

		if password == correctPassword {
			// Create session
			token := generateToken()
			sessions[token] = true
			c.SetCookie("auth_token", token, 86400*7, "/", "", true, true)
			c.Redirect(302, "/dashboard")
		} else {
			c.Redirect(302, "/login?error=1")
		}
	})

	r.GET("/logout", func(c *gin.Context) {
		token, _ := c.Cookie("auth_token")
		delete(sessions, token)
		c.SetCookie("auth_token", "", -1, "/", "", true, true)
		c.Redirect(302, "/login")
	})

	// Dashboard routes (protected)
	r.GET("/dashboard", authMiddleware(), func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(200, getDashboardHTML())
	})

	r.GET("/api/addresses", authMiddleware(), func(c *gin.Context) {
		var addresses []db.Address
		db.DB.Order("created_at DESC").Find(&addresses)
		c.JSON(200, addresses)
	})

	r.GET("/api/emails/:addressId", authMiddleware(), func(c *gin.Context) {
		var emails []db.Email
		db.DB.Where("address_id = ?", c.Param("addressId")).Order("id DESC").Find(&emails)
		c.JSON(200, emails)
	})

	r.POST("/api/addresses", authMiddleware(), func(c *gin.Context) {
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

		result := db.DB.Create(&address)
		if result.Error != nil {
			log.Printf("ERROR: Failed to create address %s via dashboard: %v", address.ID, result.Error)
			c.JSON(500, gin.H{
				"error": "Failed to create address in database",
				"details": result.Error.Error(),
			})
			return
		}
		
		log.Printf("SUCCESS: Created address %s via dashboard (expires: %s)", address.ID, address.ExpiresAt.Format(time.RFC3339))

		// Send Slack notification
		durationText := fmt.Sprintf("%d-hour", duration)
		if duration >= 24 {
			days := duration / 24
			if days == 1 {
				durationText = "24-hour"
			} else {
				durationText = fmt.Sprintf("%d-day", days)
			}
		}

		Client.PostMessage(
			os.Getenv("SLACK_CHANNEL"),
			slack.MsgOptionText("Admin created email address", false),
			slack.MsgOptionBlocks(
				slack.NewSectionBlock(
					slack.NewTextBlockObject(
						slack.MarkdownType,
						fmt.Sprintf("*👤 Admin Action*\n\nCreated email address via dashboard\n\n`%s@%s`\n\n*Duration:* %s\n*Expires:* %s", 
							address.ID, 
							os.Getenv("DOMAIN"),
							durationText,
							address.ExpiresAt.Format("Jan 2, 3:04 PM"),
						),
						false,
						false,
					),
					nil,
					nil,
				),
			),
		)

		c.JSON(200, address)
	})

	r.DELETE("/api/addresses/:id", authMiddleware(), func(c *gin.Context) {
		var address db.Address
		if err := db.DB.Where("id = ?", c.Param("id")).First(&address).Error; err != nil {
			c.JSON(404, gin.H{"error": "Address not found"})
			return
		}

		address.ExpiresAt = time.Now()
		db.DB.Save(&address)

		// Send Slack notification
		Client.PostMessage(
			os.Getenv("SLACK_CHANNEL"),
			slack.MsgOptionText("Admin deactivated email address", false),
			slack.MsgOptionBlocks(
				slack.NewSectionBlock(
					slack.NewTextBlockObject(
						slack.MarkdownType,
						fmt.Sprintf("*👤 Admin Action*\n\nDeactivated email address via dashboard\n\n`%s@%s`", 
							address.ID, 
							os.Getenv("DOMAIN"),
						),
						false,
						false,
					),
					nil,
					nil,
				),
			),
		)

		c.JSON(200, gin.H{"success": true})
	})

	// Mailgun webhook endpoints (MUST be before /:email catch-all route)
	r.POST("/webhook/mailgun", mailgun.HandleWebhook)
	r.POST("/webhook/mailgun/raw", mailgun.HandleRawWebhook)

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

	// Debug endpoint to check if address exists
	r.GET("/api/check/:addressId", func(c *gin.Context) {
		addressId := c.Param("addressId")
		var address db.Address
		tx := db.DB.Where("id = ? AND expires_at > NOW()", addressId).First(&address)
		
		if tx.Error == gorm.ErrRecordNotFound {
			c.JSON(404, gin.H{
				"found": false,
				"error": "address not found or expired",
				"id": addressId,
			})
			return
		} else if tx.Error != nil {
			c.JSON(500, gin.H{
				"found": false,
				"error": tx.Error.Error(),
				"id": addressId,
			})
			return
		}
		
		c.JSON(200, gin.H{
			"found": true,
			"id": address.ID,
			"created_at": address.CreatedAt,
			"expires_at": address.ExpiresAt,
			"user": address.User,
			"is_active": address.ExpiresAt.After(time.Now()),
		})
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
    <title>TempMail Dashboard</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        :root {
            --bg: #ffffff;
            --bg-secondary: #f8f9fa;
            --surface: #ffffff;
            --surface-hover: #f1f3f5;
            --primary: #000000;
            --primary-hover: #1a1a1a;
            --text: #1a1a1a;
            --text-secondary: #6c757d;
            --success: #28a745;
            --danger: #dc3545;
            --border: #dee2e6;
            --shadow: rgba(0, 0, 0, 0.05);
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', 'Helvetica Neue', sans-serif;
            background: var(--bg-secondary);
            color: var(--text);
            line-height: 1.6;
            min-height: 100vh;
        }

        .nav {
            background: var(--surface);
            border-bottom: 1px solid var(--border);
            padding: 1rem 0;
            margin-bottom: 2rem;
        }

        .nav-container {
            max-width: 1200px;
            margin: 0 auto;
            padding: 0 2rem;
            display: flex;
            align-items: center;
            justify-content: space-between;
        }

        .logo {
            display: flex;
            align-items: center;
            gap: 0.75rem;
            font-size: 1.25rem;
            font-weight: 600;
            color: var(--text);
        }

        .logo-img {
            width: 40px;
            height: 40px;
            border-radius: 8px;
        }

        .logout-btn {
            background: transparent;
            border: 1px solid var(--border);
            color: var(--text);
            padding: 0.5rem 1rem;
            border-radius: 6px;
            font-size: 0.875rem;
            cursor: pointer;
            transition: all 0.2s;
        }

        .logout-btn:hover {
            background: var(--surface-hover);
            transform: none;
            box-shadow: none;
        }

        .container {
            max-width: 1200px;
            margin: 0 auto;
            padding: 0 2rem 2rem 2rem;
        }

        h1 {
            font-size: 2rem;
            font-weight: 700;
            color: var(--text);
            margin-bottom: 0.5rem;
        }

        .subtitle {
            color: var(--text-secondary);
            font-size: 1rem;
            margin-bottom: 2rem;
        }

        .stats {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmin(180px, 1fr));
            gap: 1rem;
            margin-bottom: 2rem;
        }

        .stat-card {
            background: var(--surface);
            padding: 1.5rem;
            border-radius: 12px;
            border: 1px solid var(--border);
            box-shadow: 0 1px 3px var(--shadow);
        }

        .stat-label {
            color: var(--text-secondary);
            font-size: 0.75rem;
            font-weight: 500;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            margin-bottom: 0.5rem;
        }

        .stat-value {
            font-size: 2rem;
            font-weight: 700;
            color: var(--text);
        }

        .create-section {
            background: var(--surface);
            padding: 2rem;
            border-radius: 12px;
            border: 1px solid var(--border);
            margin-bottom: 2rem;
            box-shadow: 0 1px 3px var(--shadow);
        }

        .section-header {
            margin-bottom: 1.5rem;
        }

        .section-header h2 {
            font-size: 1.125rem;
            font-weight: 600;
            color: var(--text);
            margin-bottom: 0.25rem;
        }

        .section-header p {
            font-size: 0.875rem;
            color: var(--text-secondary);
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
            color: var(--text);
            font-size: 0.875rem;
            margin-bottom: 0.5rem;
            font-weight: 500;
        }

        input, select {
            background: var(--bg);
            border: 1px solid var(--border);
            border-radius: 6px;
            padding: 0.625rem 0.875rem;
            color: var(--text);
            font-size: 0.9375rem;
            transition: all 0.2s;
        }

        input:focus, select:focus {
            outline: none;
            border-color: var(--text);
            box-shadow: 0 0 0 3px rgba(0, 0, 0, 0.05);
        }

        input::placeholder {
            color: #adb5bd;
        }

        button {
            background: var(--primary);
            color: white;
            border: none;
            padding: 0.625rem 1.5rem;
            border-radius: 6px;
            font-weight: 500;
            cursor: pointer;
            transition: all 0.2s;
            font-size: 0.9375rem;
            white-space: nowrap;
        }

        button:hover {
            background: var(--primary-hover);
        }

        button:active {
            transform: scale(0.98);
        }

        .addresses-section {
            background: var(--surface);
            border-radius: 12px;
            border: 1px solid var(--border);
            padding: 2rem;
            box-shadow: 0 1px 3px var(--shadow);
        }

        .addresses-section h2 {
            font-size: 1.125rem;
            font-weight: 600;
            margin-bottom: 1.5rem;
            color: var(--text);
        }

        .addresses-grid {
            display: grid;
            gap: 1rem;
        }

        .address-card {
            background: var(--bg-secondary);
            border: 1px solid var(--border);
            border-radius: 8px;
            padding: 1.25rem;
            transition: all 0.2s ease;
            cursor: pointer;
        }

        .address-card:hover {
            box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
            border-color: var(--primary);
        }

        .address-card.expanded {
            background: var(--surface);
            cursor: default;
        }

        .address-header {
            display: flex;
            justify-content: space-between;
            align-items: start;
            margin-bottom: 0.75rem;
        }

        .address-email {
            font-size: 1rem;
            font-weight: 600;
            font-family: 'SF Mono', 'Monaco', 'Consolas', monospace;
            color: var(--text);
            word-break: break-all;
        }

        .address-actions {
            display: flex;
            gap: 0.5rem;
        }

        .btn-icon {
            background: transparent;
            border: 1px solid var(--border);
            padding: 0.375rem 0.5rem;
            border-radius: 4px;
            cursor: pointer;
            transition: all 0.2s;
            font-size: 1rem;
        }

        .btn-icon:hover {
            background: var(--danger);
            border-color: var(--danger);
            color: white;
        }

        .address-meta {
            display: flex;
            gap: 1.5rem;
            color: var(--text-secondary);
            font-size: 0.8125rem;
            flex-wrap: wrap;
        }

        .meta-item {
            display: flex;
            align-items: center;
            gap: 0.375rem;
        }

        .badge {
            display: inline-block;
            padding: 0.125rem 0.5rem;
            border-radius: 4px;
            font-size: 0.6875rem;
            font-weight: 600;
            text-transform: uppercase;
            letter-spacing: 0.025em;
        }

        .badge-active {
            background: #d4edda;
            color: #155724;
            border: 1px solid #c3e6cb;
        }

        .badge-expired {
            background: #f8d7da;
            color: #721c24;
            border: 1px solid #f5c6cb;
        }

        .loading {
            text-align: center;
            padding: 3rem;
            color: var(--text-secondary);
        }

        .spinner {
            border: 2px solid var(--border);
            border-top: 2px solid var(--primary);
            border-radius: 50%;
            width: 32px;
            height: 32px;
            animation: spin 0.8s linear infinite;
            margin: 0 auto 1rem;
        }

        @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
        }

        .empty-state {
            text-align: center;
            padding: 3rem 2rem;
            color: var(--text-secondary);
        }

        .empty-icon {
            font-size: 3rem;
            margin-bottom: 1rem;
            opacity: 0.4;
        }

        .empty-state p {
            font-size: 0.9375rem;
        }

        .emails-section {
            margin-top: 1rem;
            padding-top: 1rem;
            border-top: 1px solid var(--border);
        }

        .emails-section h3 {
            font-size: 0.875rem;
            font-weight: 600;
            color: var(--text-secondary);
            margin-bottom: 0.75rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }

        .email-item {
            background: var(--bg-secondary);
            border: 1px solid var(--border);
            border-radius: 6px;
            padding: 1rem;
            margin-bottom: 0.75rem;
            transition: all 0.2s;
        }

        .email-item:hover {
            border-color: var(--text-secondary);
        }

        .email-header {
            display: flex;
            justify-content: space-between;
            align-items: start;
            margin-bottom: 0.5rem;
        }

        .email-from {
            font-size: 0.8125rem;
            color: var(--text-secondary);
            font-family: 'SF Mono', 'Monaco', 'Consolas', monospace;
        }

        .email-time {
            font-size: 0.75rem;
            color: var(--text-secondary);
        }

        .email-subject {
            font-weight: 600;
            color: var(--text);
            margin-bottom: 0.5rem;
            font-size: 0.9375rem;
        }

        .email-preview {
            font-size: 0.875rem;
            color: var(--text-secondary);
            line-height: 1.5;
            max-height: 3rem;
            overflow: hidden;
            text-overflow: ellipsis;
            display: -webkit-box;
            -webkit-line-clamp: 2;
            -webkit-box-orient: vertical;
        }

        .email-actions {
            margin-top: 0.75rem;
            display: flex;
            gap: 0.5rem;
        }

        .btn-view {
            background: transparent;
            border: 1px solid var(--border);
            color: var(--text);
            padding: 0.375rem 0.75rem;
            border-radius: 4px;
            font-size: 0.8125rem;
            cursor: pointer;
            transition: all 0.2s;
        }

        .btn-view:hover {
            background: var(--primary);
            border-color: var(--primary);
            color: white;
        }

        .no-emails {
            text-align: center;
            padding: 2rem;
            color: var(--text-secondary);
            font-size: 0.875rem;
        }

        .email-count {
            display: inline-block;
            background: var(--primary);
            color: white;
            font-size: 0.75rem;
            font-weight: 600;
            padding: 0.125rem 0.5rem;
            border-radius: 12px;
            margin-left: 0.5rem;
        }

        .toggle-emails {
            background: transparent;
            border: 1px solid var(--border);
            color: var(--text);
            padding: 0.375rem 0.75rem;
            border-radius: 4px;
            font-size: 0.8125rem;
            cursor: pointer;
            transition: all 0.2s;
            margin-top: 0.5rem;
        }

        .toggle-emails:hover {
            background: var(--surface-hover);
        }
    </style>
</head>
<body>
    <nav class="nav">
        <div class="nav-container">
            <div class="logo">
                <img src="https://v3b.fal.media/files/b/panda/ifq-LOWzwSDnK0P1EEseI_f6b0255876734a108198875da17b3b77.jpg" alt="TempMail" class="logo-img">
                <span>TempMail</span>
            </div>
            <button class="logout-btn" onclick="window.location.href='/logout'">Logout</button>
        </div>
    </nav>

    <div class="container">
        <h1>Dashboard</h1>
        <p class="subtitle">Manage your temporary email addresses</p>

        <div class="stats" id="stats">
            <div class="stat-card">
                <div class="stat-label">Total</div>
                <div class="stat-value" id="totalAddresses">-</div>
            </div>
            <div class="stat-card">
                <div class="stat-label">Active</div>
                <div class="stat-value" id="activeAddresses">-</div>
            </div>
            <div class="stat-card">
                <div class="stat-label">Emails</div>
                <div class="stat-value" id="totalEmails">-</div>
            </div>
        </div>

        <div class="create-section">
            <div class="section-header">
                <h2>Create New Address</h2>
                <p>Generate a temporary email address</p>
            </div>
            <form class="create-form" id="createForm">
                <div class="form-group">
                    <label for="name">Custom Name</label>
                    <input type="text" id="name" placeholder="github, work, testing...">
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
                <button type="submit">Create</button>
            </form>
        </div>

        <div class="addresses-section">
            <h2>Email Addresses</h2>
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
                container.innerHTML = '<div class="empty-state"><div class="empty-icon">📭</div><p>No addresses yet. Create one above to get started.</p></div>';
                return;
            }
            
            const now = new Date();
            let html = '';
            
            for (const addr of addresses) {
                const isActive = new Date(addr.ExpiresAt) > now;
                const created = new Date(addr.CreatedAt).toLocaleString('en-US', { 
                    month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' 
                });
                const expires = new Date(addr.ExpiresAt).toLocaleString('en-US', { 
                    month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' 
                });
                
                // Get email count
                const emailRes = await fetch(API_BASE + '/api/emails/' + addr.ID);
                const emails = await emailRes.json();
                const emailCount = emails ? emails.length : 0;
                
                html += '<div class="address-card" id="addr-' + addr.ID + '">' +
                    '<div class="address-header">' +
                        '<div class="address-email">' + addr.ID + '@' + window.location.hostname.replace('mail.', '') + 
                        (emailCount > 0 ? '<span class="email-count">' + emailCount + '</span>' : '') +
                        '</div>' +
                        '<div class="address-actions">' +
                            '<button class="btn-icon" onclick="event.stopPropagation(); deleteAddress(\'' + addr.ID + '\')" title="Delete">×</button>' +
                        '</div>' +
                    '</div>' +
                    '<div class="address-meta">' +
                        '<div class="meta-item">' +
                            '<span class="badge ' + (isActive ? 'badge-active' : 'badge-expired') + '">' + (isActive ? 'Active' : 'Expired') + '</span>' +
                        '</div>' +
                        '<div class="meta-item">Created ' + created + '</div>' +
                        '<div class="meta-item">Expires ' + expires + '</div>' +
                    '</div>';
                
                if (emailCount > 0) {
                    html += '<button class="toggle-emails" onclick="toggleEmails(\'' + addr.ID + '\')">View ' + emailCount + ' Email' + (emailCount !== 1 ? 's' : '') + ' ▼</button>';
                    html += '<div class="emails-section" id="emails-' + addr.ID + '" style="display: none;"></div>';
                }
                
                html += '</div>';
            }
            
            container.innerHTML = html;
        }

        async function toggleEmails(addressId) {
            const emailsSection = document.getElementById('emails-' + addressId);
            const card = document.getElementById('addr-' + addressId);
            
            if (emailsSection.style.display === 'none') {
                // Load and show emails
                emailsSection.innerHTML = '<div class="loading"><div class="spinner"></div><p>Loading emails...</p></div>';
                emailsSection.style.display = 'block';
                card.classList.add('expanded');
                
                const res = await fetch(API_BASE + '/api/emails/' + addressId);
                const emails = await res.json();
                
                if (emails && emails.length > 0) {
                    let emailsHtml = '<h3>Received Emails</h3>';
                    emails.forEach(email => {
                        // Parse the email content to extract subject and preview
                        const emailDate = new Date(email.CreatedAt).toLocaleString('en-US', { 
                            month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' 
                        });
                        
                        emailsHtml += '<div class="email-item">' +
                            '<div class="email-header">' +
                                '<div class="email-time">' + emailDate + '</div>' +
                            '</div>' +
                            '<div class="email-actions">' +
                                '<button class="btn-view" onclick="window.open(\'/' + email.ID + '\', \'_blank\')">View Full Email</button>' +
                            '</div>' +
                        '</div>';
                    });
                    emailsSection.innerHTML = emailsHtml;
                } else {
                    emailsSection.innerHTML = '<div class="no-emails">No emails received yet</div>';
                }
                
                // Update button text
                const btn = card.querySelector('.toggle-emails');
                btn.textContent = 'Hide Emails ▲';
            } else {
                // Hide emails
                emailsSection.style.display = 'none';
                card.classList.remove('expanded');
                
                // Update button text
                const btn = card.querySelector('.toggle-emails');
                const match = btn.textContent.match(/\d+/);
                const count = match ? parseInt(match[0]) : 0;
                btn.textContent = 'View ' + count + ' Email' + (count !== 1 ? 's' : '') + ' ▼';
            }
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

func getLoginHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Login - TempMail</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', 'Helvetica Neue', sans-serif;
            background: #f8f9fa;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
        }

        .login-container {
            width: 100%;
            max-width: 400px;
            padding: 2rem;
        }

        .login-card {
            background: white;
            border: 1px solid #dee2e6;
            border-radius: 12px;
            padding: 2.5rem;
            box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
        }

        .logo {
            display: flex;
            justify-content: center;
            margin-bottom: 1.5rem;
        }

        .logo img {
            width: 64px;
            height: 64px;
            border-radius: 12px;
        }

        h1 {
            font-size: 1.5rem;
            font-weight: 600;
            text-align: center;
            color: #1a1a1a;
            margin-bottom: 0.5rem;
        }

        .subtitle {
            text-align: center;
            color: #6c757d;
            margin-bottom: 2rem;
            font-size: 0.875rem;
        }

        .form-group {
            margin-bottom: 1.5rem;
        }

        label {
            display: block;
            color: #1a1a1a;
            font-size: 0.875rem;
            margin-bottom: 0.5rem;
            font-weight: 500;
        }

        input[type="password"] {
            width: 100%;
            background: white;
            border: 1px solid #dee2e6;
            border-radius: 6px;
            padding: 0.625rem 0.875rem;
            color: #1a1a1a;
            font-size: 0.9375rem;
            transition: all 0.2s;
        }

        input[type="password"]:focus {
            outline: none;
            border-color: #1a1a1a;
            box-shadow: 0 0 0 3px rgba(0, 0, 0, 0.05);
        }

        .login-btn {
            width: 100%;
            background: #000000;
            color: white;
            border: none;
            padding: 0.75rem 1.5rem;
            border-radius: 6px;
            font-weight: 500;
            font-size: 0.9375rem;
            cursor: pointer;
            transition: all 0.3s ease;
            box-shadow: 0 4px 20px rgba(99, 102, 241, 0.3);
        }

        .login-btn:hover {
            background: #1a1a1a;
        }

        .login-btn:active {
            transform: scale(0.98);
        }

        .error {
            background: #f8d7da;
            border: 1px solid #f5c6cb;
            color: #721c24;
            padding: 0.75rem 1rem;
            border-radius: 6px;
            margin-bottom: 1.5rem;
            text-align: center;
            font-size: 0.875rem;
        }
    </style>
</head>
<body>
    <div class="login-container">
        <div class="login-card">
            <div class="logo">
                <img src="https://v3b.fal.media/files/b/panda/ifq-LOWzwSDnK0P1EEseI_f6b0255876734a108198875da17b3b77.jpg" alt="TempMail">
            </div>
            <h1>TempMail</h1>
            <p class="subtitle">Sign in to your dashboard</p>
            
            <script>
                const params = new URLSearchParams(window.location.search);
                if (params.get('error') === '1') {
                    document.write('<div class="error">Incorrect password. Please try again.</div>');
                }
            </script>

            <form method="POST" action="/login">
                <div class="form-group">
                    <label for="password">Password</label>
                    <input type="password" id="password" name="password" placeholder="Enter password" required autofocus>
                </div>
                
                <button type="submit" class="login-btn">
                    Sign In
                </button>
            </form>
        </div>
    </div>
</body>
</html>`
}
