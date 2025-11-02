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
		db.DB.Where("address_id = ?", c.Param("addressId")).Order("created_at DESC").Find(&emails)
		c.JSON(200, emails)
	})

	r.GET("/api/email/:emailId", authMiddleware(), func(c *gin.Context) {
		var email db.Email
		if err := db.DB.Where("id = ?", c.Param("emailId")).First(&email).Error; err != nil {
			c.JSON(404, gin.H{"error": "Email not found"})
			return
		}
		c.JSON(200, email)
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

		// Dashboard addresses do NOT send to Slack
		// Admin can view emails directly in the dashboard

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

		// Only send Slack notification if address was created via Slack (has timestamp)
		if address.Timestamp != "" {
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
		}

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
    <title>TempMail - Inbox</title>
    <link href="https://fonts.googleapis.com/css2?family=Google+Sans:wght@400;500;700&family=Roboto:wght@300;400;500;700&display=swap" rel="stylesheet">
    <link href="https://fonts.googleapis.com/icon?family=Material+Icons" rel="stylesheet">
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        :root {
            --primary: #1a73e8;
            --primary-hover: #1765cc;
            --primary-light: #e8f0fe;
            --danger: #d93025;
            --danger-hover: #c5221f;
            --success: #1e8e3e;
            --warning: #f9ab00;
            --bg: #ffffff;
            --bg-secondary: #f5f5f5;
            --sidebar-bg: #f8f9fa;
            --surface: #ffffff;
            --surface-hover: #f5f5f5;
            --text: #202124;
            --text-secondary: #5f6368;
            --text-light: #80868b;
            --border: #dadce0;
            --divider: #e8eaed;
            --shadow-sm: 0 1px 2px 0 rgba(60,64,67,0.3), 0 1px 3px 1px rgba(60,64,67,0.15);
            --shadow-md: 0 1px 3px 0 rgba(60,64,67,0.3), 0 4px 8px 3px rgba(60,64,67,0.15);
            --shadow-lg: 0 2px 6px rgba(0,0,0,0.06), 0 8px 24px rgba(0,0,0,0.12);
            --transition: all 0.2s cubic-bezier(0.4, 0.0, 0.2, 1);
        }

        body {
            font-family: 'Roboto', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
            background: var(--bg);
            color: var(--text);
            line-height: 1.5;
            min-height: 100vh;
            overflow: hidden;
        }

        /* Main Layout */
        .app-container {
            display: flex;
            height: 100vh;
            overflow: hidden;
        }

        /* Header */
        .header {
            position: fixed;
            top: 0;
            left: 0;
            right: 0;
            height: 64px;
            background: var(--surface);
            border-bottom: 1px solid var(--border);
            display: flex;
            align-items: center;
            padding: 0 16px;
            gap: 16px;
            z-index: 100;
        }

        .menu-icon {
            width: 48px;
            height: 48px;
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
            cursor: pointer;
            transition: var(--transition);
            color: var(--text-secondary);
        }

        .menu-icon:hover {
            background: var(--surface-hover);
        }

        .logo-section {
            display: flex;
            align-items: center;
            gap: 12px;
            margin-right: auto;
        }

        .logo-img {
            width: 40px;
            height: 40px;
            border-radius: 8px;
        }

        .logo-text {
            font-family: 'Google Sans', sans-serif;
            font-size: 22px;
            font-weight: 400;
            color: var(--text);
            letter-spacing: -0.5px;
        }

        .search-bar {
            flex: 1;
            max-width: 720px;
            position: relative;
        }

        .search-input {
            width: 100%;
            height: 48px;
            background: var(--bg-secondary);
            border: none;
            border-radius: 8px;
            padding: 0 16px 0 52px;
            font-size: 16px;
            color: var(--text);
            transition: var(--transition);
        }

        .search-input:focus {
            outline: none;
            box-shadow: var(--shadow-sm);
            background: var(--surface);
        }

        .search-icon {
            position: absolute;
            left: 16px;
            top: 50%;
            transform: translateY(-50%);
            color: var(--text-secondary);
            pointer-events: none;
        }

        .header-actions {
            display: flex;
            gap: 8px;
            align-items: center;
        }

        .icon-btn {
            width: 48px;
            height: 48px;
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
            cursor: pointer;
            transition: var(--transition);
            color: var(--text-secondary);
            border: none;
            background: transparent;
        }

        .icon-btn:hover {
            background: var(--surface-hover);
        }

        .user-avatar {
            width: 32px;
            height: 32px;
            border-radius: 50%;
            background: var(--primary);
            color: white;
            display: flex;
            align-items: center;
            justify-content: center;
            font-weight: 500;
            font-size: 14px;
            cursor: pointer;
            margin-left: 8px;
        }

        /* Sidebar */
        .sidebar {
            width: 256px;
            background: var(--bg);
            padding: 8px;
            margin-top: 64px;
            border-right: 1px solid var(--border);
            overflow-y: auto;
            flex-shrink: 0;
            transition: var(--transition);
        }

        .sidebar.collapsed {
            width: 80px;
        }

        .compose-btn {
            display: flex;
            align-items: center;
            gap: 16px;
            background: var(--primary-light);
            color: var(--primary);
            border: none;
            border-radius: 16px;
            padding: 12px 24px;
            font-size: 14px;
            font-weight: 500;
            cursor: pointer;
            margin-bottom: 8px;
            transition: var(--transition);
            box-shadow: var(--shadow-sm);
            width: 100%;
            justify-content: flex-start;
        }

        .compose-btn:hover {
            box-shadow: var(--shadow-md);
            background: var(--primary);
            color: white;
        }

        .compose-btn .material-icons {
            font-size: 24px;
        }

        .sidebar.collapsed .compose-btn {
            padding: 12px;
            justify-content: center;
        }

        .sidebar.collapsed .compose-btn span:not(.material-icons) {
            display: none;
        }

        .nav-item {
            display: flex;
            align-items: center;
            gap: 16px;
            padding: 8px 12px 8px 16px;
            border-radius: 0 24px 24px 0;
            margin-bottom: 2px;
            cursor: pointer;
            transition: var(--transition);
            color: var(--text);
            text-decoration: none;
            font-size: 14px;
            font-weight: 500;
        }

        .nav-item:hover {
            background: var(--surface-hover);
        }

        .nav-item.active {
            background: var(--primary-light);
            color: var(--primary);
        }

        .nav-item .material-icons {
            font-size: 20px;
            width: 20px;
        }

        .nav-item .badge {
            margin-left: auto;
            background: var(--text-light);
            color: white;
            padding: 2px 8px;
            border-radius: 12px;
            font-size: 12px;
            font-weight: 500;
        }

        .nav-item.active .badge {
            background: var(--primary);
        }

        .sidebar.collapsed .nav-item span:not(.material-icons) {
            display: none;
        }

        .sidebar.collapsed .nav-item {
            justify-content: center;
            border-radius: 12px;
        }

        /* Main Content */
        .main-content {
            flex: 1;
            margin-top: 64px;
            display: flex;
            overflow: hidden;
        }

        /* Email List */
        .email-list {
            width: 360px;
            background: var(--surface);
            border-right: 1px solid var(--border);
            overflow-y: auto;
            flex-shrink: 0;
        }

        .list-header {
            padding: 16px 20px;
            border-bottom: 1px solid var(--border);
            display: flex;
            align-items: center;
            justify-content: space-between;
            position: sticky;
            top: 0;
            background: var(--surface);
            z-index: 10;
        }

        .list-header h2 {
            font-family: 'Google Sans', sans-serif;
            font-size: 18px;
            font-weight: 500;
            color: var(--text);
        }

        .list-actions {
            display: flex;
            gap: 4px;
        }

        .email-item {
            display: flex;
            padding: 12px 20px;
            border-bottom: 1px solid var(--divider);
            cursor: pointer;
            transition: var(--transition);
            position: relative;
        }

        .email-item:hover {
            box-shadow: inset 1px 0 0 var(--border), inset -1px 0 0 var(--border), 0 1px 2px 0 rgba(60,64,67,0.3), 0 1px 3px 1px rgba(60,64,67,0.15);
            z-index: 1;
        }

        .email-item.active {
            background: var(--primary-light);
            border-right: 3px solid var(--primary);
        }

        .email-item.unread {
            background: #f9fafb;
            font-weight: 500;
        }

        .email-avatar {
            width: 40px;
            height: 40px;
            border-radius: 50%;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            display: flex;
            align-items: center;
            justify-content: center;
            font-weight: 500;
            font-size: 14px;
            flex-shrink: 0;
            margin-right: 12px;
        }

        .email-content {
            flex: 1;
            min-width: 0;
        }

        .email-header-row {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 4px;
        }

        .email-address {
            font-size: 14px;
            font-weight: 500;
            color: var(--text);
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
        }

        .email-time {
            font-size: 12px;
            color: var(--text-light);
            white-space: nowrap;
            margin-left: 8px;
        }

        .email-count {
            display: inline-flex;
            align-items: center;
            justify-content: center;
            min-width: 24px;
            height: 20px;
            background: var(--primary);
            color: white;
            border-radius: 10px;
            font-size: 12px;
            font-weight: 500;
            padding: 0 6px;
            margin-left: 8px;
        }

        .email-meta {
            font-size: 12px;
            color: var(--text-light);
            display: flex;
            gap: 8px;
            align-items: center;
        }

        .status-badge {
            display: inline-flex;
            align-items: center;
            padding: 2px 8px;
            border-radius: 4px;
            font-size: 11px;
            font-weight: 500;
            text-transform: uppercase;
        }

        .status-badge.active {
            background: #e6f4ea;
            color: var(--success);
        }

        .status-badge.expired {
            background: #fce8e6;
            color: var(--danger);
        }

        /* Email Preview Pane */
        .email-preview {
            flex: 1;
            background: var(--surface);
            overflow-y: auto;
            display: flex;
            flex-direction: column;
        }

        .preview-header {
            padding: 24px 32px;
            border-bottom: 1px solid var(--border);
            background: var(--surface);
            position: sticky;
            top: 0;
            z-index: 10;
        }

        .preview-title {
            font-family: 'Google Sans', sans-serif;
            font-size: 24px;
            font-weight: 400;
            color: var(--text);
            margin-bottom: 16px;
            word-break: break-all;
        }

        .preview-meta {
            display: flex;
            gap: 16px;
            flex-wrap: wrap;
            align-items: center;
        }

        .meta-item {
            display: flex;
            align-items: center;
            gap: 8px;
            font-size: 14px;
            color: var(--text-secondary);
        }

        .meta-item .material-icons {
            font-size: 18px;
        }

        .action-buttons {
            display: flex;
            gap: 8px;
            margin-top: 16px;
        }

        .btn {
            display: inline-flex;
            align-items: center;
            gap: 8px;
            padding: 10px 24px;
            border-radius: 4px;
            border: none;
            font-size: 14px;
            font-weight: 500;
            cursor: pointer;
            transition: var(--transition);
        }

        .btn-primary {
            background: var(--primary);
            color: white;
        }

        .btn-primary:hover {
            background: var(--primary-hover);
            box-shadow: var(--shadow-sm);
        }

        .btn-outline {
            background: transparent;
            border: 1px solid var(--border);
            color: var(--text);
        }

        .btn-outline:hover {
            background: var(--surface-hover);
        }

        .btn-danger {
            background: var(--danger);
            color: white;
        }

        .btn-danger:hover {
            background: var(--danger-hover);
        }

        .emails-container {
            padding: 24px 32px;
        }

        .received-email {
            background: var(--surface);
            border: 1px solid var(--border);
            border-radius: 8px;
            margin-bottom: 16px;
            overflow: hidden;
            transition: var(--transition);
        }

        .received-email:hover {
            box-shadow: var(--shadow-sm);
        }

        .received-email-header {
            padding: 16px 20px;
            background: var(--bg-secondary);
            border-bottom: 1px solid var(--border);
            display: flex;
            justify-content: space-between;
            align-items: center;
        }

        .received-email-info {
            display: flex;
            align-items: center;
            gap: 12px;
        }

        .received-email-from {
            font-weight: 500;
            color: var(--text);
            font-size: 14px;
        }

        .received-email-time {
            font-size: 13px;
            color: var(--text-light);
        }

        .received-email-body {
            padding: 20px;
            display: none;
        }

        .received-email-body.expanded {
            display: block;
        }

        .email-iframe {
            width: 100%;
            border: none;
            border-radius: 4px;
            background: white;
            min-height: 400px;
        }

        .empty-state {
            flex: 1;
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            padding: 48px;
            text-align: center;
        }

        .empty-icon {
            font-size: 96px;
            color: var(--text-light);
            margin-bottom: 24px;
            opacity: 0.3;
        }

        .empty-title {
            font-family: 'Google Sans', sans-serif;
            font-size: 20px;
            font-weight: 400;
            color: var(--text);
            margin-bottom: 8px;
        }

        .empty-text {
            font-size: 14px;
            color: var(--text-secondary);
        }

        /* Compose Modal */
        .modal-overlay {
            position: fixed;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: rgba(0, 0, 0, 0.5);
            display: none;
            align-items: center;
            justify-content: center;
            z-index: 1000;
            animation: fadeIn 0.2s ease;
        }

        .modal-overlay.active {
            display: flex;
        }

        @keyframes fadeIn {
            from { opacity: 0; }
            to { opacity: 1; }
        }

        .modal {
            background: var(--surface);
            border-radius: 8px;
            width: 90%;
            max-width: 568px;
            box-shadow: var(--shadow-lg);
            animation: slideUp 0.3s cubic-bezier(0.4, 0.0, 0.2, 1);
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

        .modal-header {
            padding: 24px 24px 16px;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }

        .modal-title {
            font-family: 'Google Sans', sans-serif;
            font-size: 22px;
            font-weight: 400;
            color: var(--text);
        }

        .modal-close {
            width: 40px;
            height: 40px;
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
            cursor: pointer;
            transition: var(--transition);
            color: var(--text-secondary);
            border: none;
            background: transparent;
        }

        .modal-close:hover {
            background: var(--surface-hover);
        }

        .modal-body {
            padding: 8px 24px 24px;
        }

        .form-field {
            margin-bottom: 24px;
        }

        .form-label {
            display: block;
            font-size: 14px;
            font-weight: 500;
            color: var(--text);
            margin-bottom: 8px;
        }

        .form-input,
        .form-select {
            width: 100%;
            padding: 12px 16px;
            border: 1px solid var(--border);
            border-radius: 4px;
            font-size: 14px;
            color: var(--text);
            background: var(--surface);
            transition: var(--transition);
        }

        .form-input:focus,
        .form-select:focus {
            outline: none;
            border-color: var(--primary);
            box-shadow: 0 0 0 1px var(--primary);
        }

        .form-input::placeholder {
            color: var(--text-light);
        }

        .form-helper {
            font-size: 12px;
            color: var(--text-secondary);
            margin-top: 6px;
        }

        .modal-footer {
            padding: 16px 24px;
            border-top: 1px solid var(--border);
            display: flex;
            justify-content: flex-end;
            gap: 8px;
        }

        /* Loading States */
        .loading-spinner {
            display: inline-block;
            width: 20px;
            height: 20px;
            border: 2px solid var(--border);
            border-top-color: var(--primary);
            border-radius: 50%;
            animation: spin 0.8s linear infinite;
        }

        @keyframes spin {
            to { transform: rotate(360deg); }
        }

        .loading-overlay {
            position: absolute;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: rgba(255, 255, 255, 0.9);
            display: flex;
            align-items: center;
            justify-content: center;
            flex-direction: column;
            gap: 16px;
        }

        .loading-text {
            color: var(--text-secondary);
            font-size: 14px;
        }

        /* Utilities */
        .material-icons {
            user-select: none;
        }

        ::-webkit-scrollbar {
            width: 8px;
            height: 8px;
        }

        ::-webkit-scrollbar-track {
            background: transparent;
        }

        ::-webkit-scrollbar-thumb {
            background: var(--border);
            border-radius: 4px;
        }

        ::-webkit-scrollbar-thumb:hover {
            background: var(--text-light);
        }

        @media (max-width: 768px) {
            .sidebar {
                position: fixed;
                left: -256px;
                top: 64px;
                bottom: 0;
                z-index: 99;
                box-shadow: var(--shadow-lg);
            }

            .sidebar.open {
                left: 0;
            }

            .email-list {
                width: 100%;
            }

            .email-preview {
                position: fixed;
                left: 100%;
                top: 64px;
                right: 0;
                bottom: 0;
                transition: left 0.3s ease;
            }

            .email-preview.active {
                left: 0;
            }

            .search-bar {
                display: none;
            }
        }


    </style>
</head>
<body>
    <div class="app-container">
        <!-- Header -->
        <header class="header">
            <div class="menu-icon" onclick="toggleSidebar()">
                <span class="material-icons">menu</span>
            </div>
            <div class="logo-section">
                <img src="https://v3b.fal.media/files/b/panda/ifq-LOWzwSDnK0P1EEseI_f6b0255876734a108198875da17b3b77.jpg" alt="TempMail" class="logo-img">
                <span class="logo-text">TempMail</span>
            </div>
            <div class="search-bar">
                <span class="material-icons search-icon">search</span>
                <input type="text" class="search-input" placeholder="Search addresses..." id="searchInput" oninput="filterAddresses()">
            </div>
            <div class="header-actions">
                <button class="icon-btn" onclick="loadAddresses()" title="Refresh">
                    <span class="material-icons">refresh</span>
                </button>
                <button class="icon-btn" onclick="window.location.href='/logout'" title="Logout">
                    <span class="material-icons">logout</span>
                </button>
                <div class="user-avatar" title="Admin">A</div>
            </div>
        </header>

        <!-- Sidebar -->
        <aside class="sidebar" id="sidebar">
            <button class="compose-btn" onclick="openComposeModal()">
                <span class="material-icons">add</span>
                <span>Create Address</span>
            </button>
            <nav>
                <div class="nav-item active">
                    <span class="material-icons">inbox</span>
                    <span>Inbox</span>
                    <span class="badge" id="totalBadge">0</span>
                </div>
                <div class="nav-item" onclick="filterByStatus('active')">
                    <span class="material-icons">check_circle</span>
                    <span>Active</span>
                    <span class="badge" id="activeBadge">0</span>
                </div>
                <div class="nav-item" onclick="filterByStatus('expired')">
                    <span class="material-icons">schedule</span>
                    <span>Expired</span>
                    <span class="badge" id="expiredBadge">0</span>
                </div>
            </nav>
        </aside>

        <!-- Main Content -->
        <main class="main-content">
            <!-- Email List -->
            <div class="email-list" id="emailList">
                <div class="list-header">
                    <h2>Addresses</h2>
                    <div class="list-actions">
                        <button class="icon-btn" onclick="loadAddresses()" title="Refresh">
                            <span class="material-icons">refresh</span>
                        </button>
                    </div>
                </div>
                <div id="addressItems">
                    <div class="empty-state">
                        <div class="loading-spinner"></div>
                        <div class="loading-text">Loading addresses...</div>
                    </div>
                </div>
            </div>

            <!-- Email Preview Pane -->
            <div class="email-preview" id="emailPreview">
                <div class="empty-state">
                    <div class="empty-icon">📭</div>
                    <div class="empty-title">Select an address</div>
                    <div class="empty-text">Choose an address from the list to view emails</div>
                </div>
            </div>
        </main>
    </div>

    <!-- Compose Modal -->
    <div class="modal-overlay" id="composeModal">
        <div class="modal">
            <div class="modal-header">
                <h2 class="modal-title">Create New Address</h2>
                <button class="modal-close" onclick="closeComposeModal()">
                    <span class="material-icons">close</span>
                </button>
            </div>
            <form id="composeForm" onsubmit="createAddress(event)">
                <div class="modal-body">
                    <div class="form-field">
                        <label class="form-label" for="addressName">Custom Name (Optional)</label>
                        <input type="text" id="addressName" class="form-input" placeholder="e.g., github, newsletter, testing">
                        <div class="form-helper">Adds a prefix to your email address</div>
                    </div>
                    <div class="form-field">
                        <label class="form-label" for="addressDuration">Duration</label>
                        <select id="addressDuration" class="form-select">
                            <option value="24">24 hours</option>
                            <option value="48">2 days</option>
                            <option value="72">3 days</option>
                            <option value="168">1 week</option>
                            <option value="336">2 weeks</option>
                        </select>
                        <div class="form-helper">How long this address should remain active</div>
                    </div>
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-outline" onclick="closeComposeModal()">Cancel</button>
                    <button type="submit" class="btn btn-primary">
                        <span class="material-icons">add</span>
                        Create Address
                    </button>
                </div>
            </form>
        </div>
    </div>

    <script>
        const API_BASE = '';
        let addresses = [];
        let currentFilter = 'all';
        let selectedAddressId = null;

        // Initialize
        document.addEventListener('DOMContentLoaded', function() {
            loadAddresses();
            setInterval(loadAddresses, 30000); // Auto-refresh every 30 seconds
        });

        // Toggle Sidebar
        function toggleSidebar() {
            document.getElementById('sidebar').classList.toggle('collapsed');
        }

        // Compose Modal
        function openComposeModal() {
            document.getElementById('composeModal').classList.add('active');
        }

        function closeComposeModal() {
            document.getElementById('composeModal').classList.remove('active');
            document.getElementById('composeForm').reset();
        }

        // Create Address
        async function createAddress(e) {
            e.preventDefault();
            const name = document.getElementById('addressName').value;
            const duration = parseInt(document.getElementById('addressDuration').value);

            try {
                const res = await fetch(API_BASE + '/api/addresses', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ name, duration })
                });

                if (res.ok) {
                    closeComposeModal();
                    await loadAddresses();
                    // Auto-select the new address
                    const data = await res.json();
                    selectAddress(data.ID);
                }
            } catch (error) {
                console.error('Error creating address:', error);
            }
        }

        // Load Addresses
        async function loadAddresses() {
            try {
                const res = await fetch(API_BASE + '/api/addresses');
                addresses = await res.json();

                // Update badges
                const now = new Date();
                const activeCount = addresses.filter(a => new Date(a.ExpiresAt) > now).length;
                const expiredCount = addresses.length - activeCount;

                document.getElementById('totalBadge').textContent = addresses.length;
                document.getElementById('activeBadge').textContent = activeCount;
                document.getElementById('expiredBadge').textContent = expiredCount;

                // Render address list
                renderAddressList();
            } catch (error) {
                console.error('Error loading addresses:', error);
            }
        }

        // Render Address List
        async function renderAddressList() {
            const container = document.getElementById('addressItems');
            const now = new Date();

            // Filter addresses
            let filteredAddresses = addresses;
            if (currentFilter === 'active') {
                filteredAddresses = addresses.filter(a => new Date(a.ExpiresAt) > now);
            } else if (currentFilter === 'expired') {
                filteredAddresses = addresses.filter(a => new Date(a.ExpiresAt) <= now);
            }

            // Search filter
            const searchTerm = document.getElementById('searchInput').value.toLowerCase();
            if (searchTerm) {
                filteredAddresses = filteredAddresses.filter(a => 
                    a.ID.toLowerCase().includes(searchTerm)
                );
            }

            if (filteredAddresses.length === 0) {
                container.innerHTML = '<div class="empty-state"><div class="empty-icon">📭</div><div class="empty-title">No addresses</div><div class="empty-text">Create a new address to get started</div></div>';
                return;
            }

            // Load email counts for all addresses
            const emailCounts = {};
            for (const addr of filteredAddresses) {
                try {
                    const emailRes = await fetch(API_BASE + '/api/emails/' + addr.ID);
                    const emails = await emailRes.json();
                    emailCounts[addr.ID] = emails ? emails.length : 0;
                } catch (e) {
                    emailCounts[addr.ID] = 0;
                }
            }

            let html = '';
            for (const addr of filteredAddresses) {
                const isActive = new Date(addr.ExpiresAt) > now;
                const emailCount = emailCounts[addr.ID] || 0;
                const created = formatDate(addr.CreatedAt);
                const initial = addr.ID.charAt(0).toUpperCase();

                html += '<div class="email-item' + (addr.ID === selectedAddressId ? ' active' : '') + (emailCount > 0 ? ' unread' : '') + '" onclick="selectAddress(\'' + addr.ID + '\')">' +
                    '<div class="email-avatar">' + initial + '</div>' +
                    '<div class="email-content">' +
                        '<div class="email-header-row">' +
                            '<span class="email-address">' + addr.ID + '</span>' +
                            '<span class="email-time">' + created + '</span>' +
                        '</div>' +
                        '<div class="email-meta">' +
                            '<span class="status-badge ' + (isActive ? 'active' : 'expired') + '">' + (isActive ? 'Active' : 'Expired') + '</span>' +
                            '<span>Expires ' + formatDate(addr.ExpiresAt) + '</span>' +
                            (emailCount > 0 ? '<span class="email-count">' + emailCount + '</span>' : '') +
                        '</div>' +
                    '</div>' +
                '</div>';
            }

            container.innerHTML = html;
        }

        // Select Address and Load Preview
        async function selectAddress(addressId) {
            selectedAddressId = addressId;
            renderAddressList(); // Update active state in list

            const previewPane = document.getElementById('emailPreview');
            previewPane.innerHTML = '<div class="loading-overlay"><div class="loading-spinner"></div><div class="loading-text">Loading address details...</div></div>';

            try {
                // Find address data
                const address = addresses.find(a => a.ID === addressId);
                if (!address) return;

                const now = new Date();
                const isActive = new Date(address.ExpiresAt) > now;
                const domain = window.location.hostname.replace('mail.', '');
                const fullEmail = addressId + '@' + domain;

                // Load emails for this address
                const emailRes = await fetch(API_BASE + '/api/emails/' + addressId);
                const emails = await emailRes.json();

                // Build preview HTML
                let html = '<div class="preview-header">' +
                    '<h1 class="preview-title">' + fullEmail + '</h1>' +
                    '<div class="preview-meta">' +
                        '<div class="meta-item">' +
                            '<span class="material-icons">schedule</span>' +
                            '<span>Created ' + formatDate(address.CreatedAt) + '</span>' +
                        '</div>' +
                        '<div class="meta-item">' +
                            '<span class="material-icons">event</span>' +
                            '<span>Expires ' + formatDate(address.ExpiresAt) + '</span>' +
                        '</div>' +
                        '<div class="meta-item">' +
                            '<span class="status-badge ' + (isActive ? 'active' : 'expired') + '">' + (isActive ? 'Active' : 'Expired') + '</span>' +
                        '</div>' +
                    '</div>' +
                    '<div class="action-buttons">' +
                        '<button class="btn btn-outline" onclick="copyToClipboard(\'' + fullEmail + '\')">' +
                            '<span class="material-icons">content_copy</span> Copy Address' +
                        '</button>' +
                        '<button class="btn btn-danger" onclick="deleteAddress(\'' + addressId + '\')">' +
                            '<span class="material-icons">delete</span> Delete' +
                        '</button>' +
                    '</div>' +
                '</div>' +
                '<div class="emails-container">';

                if (!emails || emails.length === 0) {
                    html += '<div class="empty-state">' +
                        '<div class="empty-icon">📬</div>' +
                        '<div class="empty-title">No emails yet</div>' +
                        '<div class="empty-text">Emails sent to this address will appear here</div>' +
                    '</div>';
                } else {
                    html += '<h3 style="font-size: 14px; font-weight: 500; color: var(--text-secondary); margin-bottom: 16px; text-transform: uppercase; letter-spacing: 0.5px;">Received Emails (' + emails.length + ')</h3>';
                    
                    for (const email of emails) {
                        const emailDate = formatDateTime(email.CreatedAt);
                        html += '<div class="received-email" id="email-' + email.ID + '">' +
                            '<div class="received-email-header" onclick="toggleEmail(\'' + email.ID + '\')">' +
                                '<div class="received-email-info">' +
                                    '<div class="received-email-from">' +
                                        '<span class="material-icons" style="font-size: 16px; vertical-align: middle; margin-right: 4px;">email</span>' +
                                        'Email #' + email.ID.substring(0, 8) +
                                    '</div>' +
                                    '<div class="received-email-time">' + emailDate + '</div>' +
                                '</div>' +
                                '<button class="btn btn-outline" style="padding: 6px 12px; font-size: 13px;">' +
                                    '<span class="material-icons" style="font-size: 16px;">open_in_new</span> View' +
                                '</button>' +
                            '</div>' +
                            '<div class="received-email-body" id="email-body-' + email.ID + '">' +
                                '<iframe class="email-iframe" src="/' + email.ID + '" onload="resizeIframe(this)"></iframe>' +
                            '</div>' +
                        '</div>';
                    }
                }

                html += '</div>';
                previewPane.innerHTML = html;
            } catch (error) {
                console.error('Error loading address:', error);
                previewPane.innerHTML = '<div class="empty-state"><div class="empty-icon">⚠️</div><div class="empty-title">Error</div><div class="empty-text">Failed to load address details</div></div>';
            }
        }

        // Toggle Email Expand
        function toggleEmail(emailId) {
            const body = document.getElementById('email-body-' + emailId);
            body.classList.toggle('expanded');
        }

        // Resize iframe
        function resizeIframe(iframe) {
            try {
                iframe.style.height = (iframe.contentWindow.document.body.scrollHeight + 40) + 'px';
            } catch (e) {
                iframe.style.height = '400px';
            }
        }

        // Delete Address
        async function deleteAddress(id) {
            if (!confirm('Are you sure you want to delete this address? This cannot be undone.')) return;

            try {
                await fetch(API_BASE + '/api/addresses/' + id, {
                    method: 'DELETE'
                });

                selectedAddressId = null;
                document.getElementById('emailPreview').innerHTML = '<div class="empty-state"><div class="empty-icon">📭</div><div class="empty-title">Select an address</div><div class="empty-text">Choose an address from the list to view emails</div></div>';
                
                await loadAddresses();
            } catch (error) {
                console.error('Error deleting address:', error);
            }
        }

        // Filter Functions
        function filterByStatus(status) {
            currentFilter = status;
            
            // Update active nav item
            document.querySelectorAll('.nav-item').forEach(item => item.classList.remove('active'));
            event.target.closest('.nav-item').classList.add('active');
            
            renderAddressList();
        }

        function filterAddresses() {
            renderAddressList();
        }

        // Utility Functions
        function formatDate(dateString) {
            const date = new Date(dateString);
            const now = new Date();
            const diff = now - date;
            const days = Math.floor(diff / (1000 * 60 * 60 * 24));

            if (days === 0) return 'Today';
            if (days === 1) return 'Yesterday';
            if (days < 7) return days + ' days ago';

            return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
        }

        function formatDateTime(dateString) {
            const date = new Date(dateString);
            return date.toLocaleDateString('en-US', { 
                month: 'short', 
                day: 'numeric', 
                hour: '2-digit', 
                minute: '2-digit' 
            });
        }

        function copyToClipboard(text) {
            navigator.clipboard.writeText(text).then(() => {
                const btn = event.target.closest('button');
                const originalHTML = btn.innerHTML;
                btn.innerHTML = '<span class="material-icons">check</span> Copied!';
                setTimeout(() => {
                    btn.innerHTML = originalHTML;
                }, 2000);
            });
        }

        // Close modal on click outside
        document.getElementById('composeModal').addEventListener('click', function(e) {
            if (e.target === this) {
                closeComposeModal();
            }
        });
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
