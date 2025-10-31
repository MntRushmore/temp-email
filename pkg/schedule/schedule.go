package schedule

import (
	"fmt"
	"os"
	"time"

	"github.com/cjdenio/temp-email/pkg/db"
	"github.com/cjdenio/temp-email/pkg/slackevents"
	"github.com/go-co-op/gocron"
	"github.com/slack-go/slack"
)

func Start() {
	scheduler := gocron.NewScheduler(time.UTC)

	scheduler.Every(30).Minutes().Tag("expiry notification").Do(func() {
		fmt.Println("Checking for expired addresses...")

		var emails []db.Address
		tx := db.DB.Where("expires_at < NOW() AND NOT expired_message_sent").Find(&emails)
		if tx.Error != nil {
			fmt.Println(tx.Error)
		}

		fmt.Println(len(emails))

		for _, e := range emails {
			// Only send if we have a timestamp (address was created via Slack)
			if e.Timestamp != "" {
				_, _, err := slackevents.Client.PostMessage(
					os.Getenv("SLACK_CHANNEL"),
					slack.MsgOptionText("Address expired", false),
					slack.MsgOptionTS(e.Timestamp),
					slack.MsgOptionBlocks(
						slack.NewDividerBlock(),
						slack.NewSectionBlock(
							slack.NewTextBlockObject(
								slack.MarkdownType,
								"*⏰ Address Expired*\n\nThis address has reached its expiration time and will no longer receive emails.",
								false,
								false,
							),
							nil,
							nil,
						),
						slack.NewActionBlock(
							"reactivate",
							slack.NewButtonBlockElement(
								"reactivate",
								e.ID,
								slack.NewTextBlockObject(slack.PlainTextType, "Extend for 24 Hours", false, false),
							).WithStyle(slack.StylePrimary),
						),
						slack.NewDividerBlock(),
					))
				if err != nil {
					fmt.Println(err.Error())
				}
				slackevents.Client.AddReaction("clock1", slack.ItemRef{
					Channel:   os.Getenv("SLACK_CHANNEL"),
					Timestamp: e.Timestamp,
				})
			}

			e.ExpiredMessageSent = true
			db.DB.Save(&e)
		}
	})

	scheduler.StartAsync()
}
