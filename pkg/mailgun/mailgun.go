package mailgun

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/DusanKasan/parsemail"
	"github.com/PuerkitoBio/goquery"
	"github.com/cjdenio/temp-email/pkg/db"
	"github.com/cjdenio/temp-email/pkg/util"
	"github.com/gin-gonic/gin"
	"github.com/slack-go/slack"
	"gorm.io/gorm"

	md "github.com/JohannesKaufmann/html-to-markdown"
)

var SlackClient *slack.Client

// VerifyWebhookSignature verifies the Mailgun webhook signature
func VerifyWebhookSignature(timestamp, token, signature, signingKey string) bool {
	h := hmac.New(sha256.New, []byte(signingKey))
	h.Write([]byte(timestamp))
	h.Write([]byte(token))
	
	computedSignature := hex.EncodeToString(h.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(computedSignature))
}

// HandleWebhook processes incoming emails from Mailgun
func HandleWebhook(c *gin.Context) {
	// Get signature verification data
	timestamp := c.PostForm("timestamp")
	token := c.PostForm("token")
	signature := c.PostForm("signature")
	
	// Verify signature
	signingKey := os.Getenv("MAILGUN_SIGNING_KEY")
	if signingKey != "" {
		// Check timestamp to prevent replay attacks (allow 5 minute window)
		ts, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil || time.Now().Unix()-ts > 300 {
			log.Printf("Webhook rejected: timestamp too old or invalid")
			c.JSON(406, gin.H{"error": "Invalid timestamp"})
			return
		}
		
		if !VerifyWebhookSignature(timestamp, token, signature, signingKey) {
			log.Printf("Webhook rejected: invalid signature")
			c.JSON(406, gin.H{"error": "Invalid signature"})
			return
		}
	}
	
	// Extract email data
	recipient := c.PostForm("recipient")
	sender := c.PostForm("sender")
	subject := c.PostForm("subject")
	bodyPlain := c.PostForm("body-plain")
	bodyHtml := c.PostForm("body-html")
	
	log.Printf("Mailgun webhook received: to=%s from=%s subject=%s", recipient, sender, subject)
	
	// Extract address ID from recipient (format: addressId@domain)
	split := strings.Split(recipient, "@")
	if len(split) < 2 {
		log.Printf("Invalid recipient format: %s", recipient)
		c.JSON(200, gin.H{"status": "ignored"})
		return
	}
	
	addressId := split[0]
	
	// Look up address in database
	var address db.Address
	tx := db.DB.Where("id = ? AND expires_at > NOW()", addressId).First(&address)
	if tx.Error == gorm.ErrRecordNotFound {
		log.Printf("REJECT: Address not found or expired: %s (from: %s)", addressId, sender)
		c.JSON(200, gin.H{"status": "rejected", "reason": "address not found or expired"})
		return
	} else if tx.Error != nil {
		log.Printf("ERROR: Database query failed for address %s: %v", addressId, tx.Error)
		c.JSON(200, gin.H{"status": "error"})
		return
	}
	
	log.Printf("ACCEPT: Email received for %s from %s", recipient, sender)
	
	// Create email content in proper MIME format so viewer can parse it
	rawEmailContent := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/html; charset=utf-8\r\n\r\n%s", sender, recipient, subject, bodyHtml)
	
	// If no HTML body, use plain text
	if bodyHtml == "" {
		rawEmailContent = fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s", sender, recipient, subject, bodyPlain)
	}
	
	// Save email to database
	savedEmail := &db.Email{
		ID:        util.GenerateEmailAddress(),
		AddressID: address.ID,
		Content:   rawEmailContent,
	}
	
	db.DB.Create(&savedEmail)
	
	// Format subject
	subjectText := subject
	if subjectText == "" {
		subjectText = "_no subject_"
	} else {
		subjectText = fmt.Sprintf("subject: *%s*", subject)
	}
	
	// Process body
	body := ""
	if bodyHtml != "" {
		// Convert HTML to Markdown for Slack
		converter := md.NewConverter("", true, &md.Options{
			StrongDelimiter: "*",
			EmDelimiter:     "_",
		})
		
		converter.AddRules(
			md.Rule{
				Filter: []string{"a"},
				Replacement: func(content string, selec *goquery.Selection, options *md.Options) *string {
					return md.String(fmt.Sprintf("<%s|%s>", selec.AttrOr("href", content), content))
				},
			},
			md.Rule{
				Filter: []string{"h1", "h2", "h3", "h4", "h5", "h6"},
				Replacement: func(content string, selec *goquery.Selection, options *md.Options) *string {
					return md.String("\n\n*" + content + "*\n\n")
				},
			},
			md.Rule{
				Filter: []string{"img"},
				Replacement: func(content string, selec *goquery.Selection, options *md.Options) *string {
					return md.String("")
				},
			},
		)
		
		var err error
		body, err = converter.ConvertString(bodyHtml)
		if err != nil {
			log.Printf("Error converting HTML: %v", err)
			body = bodyPlain
		}
	} else {
		body = bodyPlain
	}
	
	// Post to Slack (only if address was created via Slack)
	if address.Timestamp != "" && SlackClient != nil {
		_, _, err := SlackClient.PostMessage(
			os.Getenv("SLACK_CHANNEL"),
			slack.MsgOptionDisableLinkUnfurl(),
			slack.MsgOptionDisableMediaUnfurl(),
			slack.MsgOptionTS(address.Timestamp),
			slack.MsgOptionBlocks(
				slack.NewSectionBlock(
					slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("message from `%s`\n%s", sender, util.SanitizeInput(subjectText)), false, false),
					nil,
					nil,
				),
				slack.NewDividerBlock(),
				slack.NewSectionBlock(
					slack.NewTextBlockObject("mrkdwn", util.SanitizeInput(body), false, false),
					nil,
					nil,
				),
				slack.NewContextBlock("", slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("Not rendering properly? Click <%s/%s|here> to view this email in your browser.", os.Getenv("APP_DOMAIN"), savedEmail.ID), false, false)),
			),
		)
		if err != nil {
			log.Printf("Error posting to Slack: %v", err)
		}
	}
	
	c.JSON(200, gin.H{"status": "ok"})
}

// HandleRawWebhook processes raw MIME emails from Mailgun (alternative method)
func HandleRawWebhook(c *gin.Context) {
	// Get signature verification
	timestamp := c.PostForm("timestamp")
	token := c.PostForm("token")
	signature := c.PostForm("signature")
	
	// Verify signature
	signingKey := os.Getenv("MAILGUN_SIGNING_KEY")
	if signingKey != "" {
		ts, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil || time.Now().Unix()-ts > 300 {
			log.Printf("Webhook rejected: timestamp too old or invalid")
			c.JSON(406, gin.H{"error": "Invalid timestamp"})
			return
		}
		
		if !VerifyWebhookSignature(timestamp, token, signature, signingKey) {
			log.Printf("Webhook rejected: invalid signature")
			c.JSON(406, gin.H{"error": "Invalid signature"})
			return
		}
	}
	
	// Get recipient
	recipient := c.PostForm("recipient")
	
	// Get raw MIME message
	file, err := c.FormFile("body-mime")
	if err != nil {
		log.Printf("Error getting MIME file: %v", err)
		c.JSON(400, gin.H{"error": "No MIME data"})
		return
	}
	
	// Open and read the file
	src, err := file.Open()
	if err != nil {
		log.Printf("Error opening MIME file: %v", err)
		c.JSON(500, gin.H{"error": "Cannot read MIME"})
		return
	}
	defer src.Close()
	
	rawEmail, err := io.ReadAll(src)
	if err != nil {
		log.Printf("Error reading MIME file: %v", err)
		c.JSON(500, gin.H{"error": "Cannot read MIME"})
		return
	}
	
	// Parse email
	email, err := parsemail.Parse(bytes.NewReader(rawEmail))
	if err != nil {
		log.Printf("Error parsing email: %v", err)
		c.JSON(500, gin.H{"error": "Cannot parse email"})
		return
	}
	
	// Extract address ID
	split := strings.Split(recipient, "@")
	if len(split) < 2 {
		log.Printf("Invalid recipient format: %s", recipient)
		c.JSON(200, gin.H{"status": "ignored"})
		return
	}
	
	addressId := split[0]
	
	// Look up address
	var address db.Address
	tx := db.DB.Where("id = ? AND expires_at > NOW()", addressId).First(&address)
	if tx.Error == gorm.ErrRecordNotFound {
		log.Printf("REJECT: Address not found or expired: %s (from: %s)", addressId, email.From[0].Address)
		c.JSON(200, gin.H{"status": "rejected"})
		return
	} else if tx.Error != nil {
		log.Printf("ERROR: Database query failed: %v", tx.Error)
		c.JSON(200, gin.H{"status": "error"})
		return
	}
	
	log.Printf("ACCEPT: Email received for %s from %s", recipient, email.From[0].Address)
	
	// Save to database
	savedEmail := &db.Email{
		ID:        util.GenerateEmailAddress(),
		AddressID: address.ID,
		Content:   string(rawEmail),
	}
	
	db.DB.Create(&savedEmail)
	
	// Post to Slack (same logic as regular webhook)
	// ... (rest of the Slack posting logic)
	
	c.JSON(200, gin.H{"status": "ok"})
}
