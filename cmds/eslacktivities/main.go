package main

import (
	"flag"
	"os"

	"github.com/golang/glog"
	"github.com/lukegb/eslacktivities"
)

var (
	slackToken = flag.String("slack-token", "", "Bot token for Slack API")

	eactivitiesAPIKey       = flag.String("eactivities-api-key", "", "API key for eActivities API")
	eactivitiesCentreNumber = flag.String("eactivities-centre", "605", "Club centre number in eActivities")

	facebookAppID     = flag.String("facebook-app-id", "", "App ID for Facebook API")
	facebookAppSecret = flag.String("facebook-app-secret", "", "App secret for Facebook API")
	facebookPageName  = flag.String("facebook-page-name", "ICDoCSoc", "Page name to use for event lookups")
)

func main() {
	flag.Parse()
	if *slackToken == "" || *eactivitiesAPIKey == "" || *facebookAppID == "" || *facebookAppSecret == "" {
		flag.Usage()
		os.Exit(2)
	}

	bot, err := eslacktivities.New(eslacktivities.BotOptions{
		Token: *slackToken,

		EActivitiesKey:    *eactivitiesAPIKey,
		EActivitiesCentre: *eactivitiesCentreNumber,

		FacebookAppID:     *facebookAppID,
		FacebookAppSecret: *facebookAppSecret,
		FacebookPageName:  *facebookPageName,
	})
	if err != nil {
		glog.Exitf("eslacktivities.New: %v", err)
	}
	bot.Run()
}
