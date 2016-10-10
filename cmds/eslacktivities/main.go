package main

import (
	"flag"
	"os"

	"github.com/golang/glog"
	"github.com/lukegb/eslacktivities"
)

var (
	slackToken              = flag.String("slack-token", "", "Bot token for Slack API")
	eactivitiesAPIKey       = flag.String("eactivities-api-key", "", "API key for eActivities API")
	eactivitiesCentreNumber = flag.String("eactivities-centre", "605", "Club centre number in eActivities")
)

func main() {
	flag.Parse()
	if *slackToken == "" || *eactivitiesAPIKey == "" {
		flag.Usage()
		os.Exit(2)
	}

	bot, err := eslacktivities.New(eslacktivities.BotOptions{
		Token:             *slackToken,
		EActivitiesKey:    *eactivitiesAPIKey,
		EActivitiesCentre: *eactivitiesCentreNumber,
	})
	if err != nil {
		glog.Exitf("eslacktivities.New: %v", err)
	}
	bot.Run()
}
