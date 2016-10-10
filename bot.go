package eslacktivities

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"github.com/golang/glog"
	"github.com/leekchan/accounting"
	"github.com/lukegb/egotivities"
	"github.com/nlopes/slack"
)

var (
	// ErrInvalidAuth is returned when invalid credentials are provided.
	ErrInvalidAuth = fmt.Errorf("invalid credentials")

	barNightCost = uint64(1200)
)

func formatMoney(m *big.Float) string {
	ac := accounting.Accounting{Precision: 2, Symbol: "£"}
	mint, _ := m.Uint64()
	mint = mint / barNightCost
	bnstr := fmt.Sprintf("%d %s", mint, "bar night")
	if mint != 1 {
		bnstr = bnstr + "s"
	}
	return fmt.Sprintf("%s (%s)", ac.FormatMoneyBigFloat(m), bnstr)
}

// BotOptions contains the options required to construct a Bot.
type BotOptions struct {
	Token string

	EActivitiesKey    string
	EActivitiesCentre string
}

// Bot responds to queries for eActivities data in Slack and returns useful information.
type Bot struct {
	api *slack.Client
	rtm *slack.RTM

	eactivities egotivities.Client
	centreNum   string
}

// Run initiates the Bot's message loop. It will not return, unless a fatal error occurs.
func (bot *Bot) Run() error {
	glog.Info("Starting up...")
	bot.rtm = bot.api.NewRTM()
	go bot.rtm.ManageConnection()
	glog.Info("Got RTM...")
	for {
		glog.Info("Waiting for event.")
		select {
		case msg := <-bot.rtm.IncomingEvents:
			switch ev := msg.Data.(type) {
			case *slack.RTMError:
				glog.Errorf("Error from RTM: %s", ev.Error())
			case *slack.InvalidAuthEvent:
				glog.Error("RTM fired InvalidAuthEvent")
				return ErrInvalidAuth
			case *slack.MessageEvent:
				glog.Infof("Got message in %s: %s", ev.Channel, ev.Text)
				mev := *ev
				msg := slack.Message(mev)
				go bot.handleMessage(&msg)
			default:
				glog.Infof("Got event: %#v", ev)
			}
		}
	}
}

func (bot *Bot) trimPrefix(msg *slack.Message) (string, bool) {
	myName := bot.rtm.GetInfo().User.Name
	myID := bot.rtm.GetInfo().User.ID
	allowedPrefixes := []string{
		fmt.Sprintf("%s ", myName),
		fmt.Sprintf("@%s ", myName),
		fmt.Sprintf("<@%s> ", myID),
	}
	for _, p := range allowedPrefixes {
		if strings.HasPrefix(msg.Text, p) {
			return strings.TrimPrefix(msg.Text, p), true
		}
	}
	return msg.Text, false
}

// handleMessage passes a message through the bot's message handlers.
func (bot *Bot) handleMessage(msg *slack.Message) {
	text, isMe := bot.trimPrefix(msg)
	if !isMe {
		// Ignore messages not directed at me.
		return
	}
	channel := msg.Channel
	bot.rtm.SendMessage(bot.rtm.NewTypingMessage(channel))
	resp, err := bot.handlePlainMessage(text)
	switch {
	case err != nil:
		bot.rtm.SendMessage(bot.rtm.NewOutgoingMessage(fmt.Sprintf("An error occurred: %s", err), channel))
	case resp != "":
		bot.rtm.SendMessage(bot.rtm.NewOutgoingMessage(resp, channel))
	}
}

// handlePlainMessage takes a request and returns a string response. If the response is empty, then no message is sent.
func (bot *Bot) handlePlainMessage(text string) (string, error) {
	switch text {
	case "how many members do we have", "members count", "membership":
		members, err := egotivities.Members(bot.eactivities, bot.centreNum, egotivities.CurrentYear())
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("We have *%d* members.", len(members)), nil
	case "how much money do we have":
		transactionLines, err := egotivities.TransactionLines(bot.eactivities, bot.centreNum, egotivities.CurrentYear())
		if err != nil {
			return "", err
		}
		money := &big.Float{}
		pendingMoney := &big.Float{}
		for _, l := range transactionLines {
			m := money
			if l.Pending || l.Outstanding {
				m = pendingMoney
			}
			m.Add(m, &l.Amount.Float)
		}
		return fmt.Sprintf("We have *%s* (with an extra *%s* on the way) in the bank.", formatMoney(money), formatMoney(pendingMoney)), nil
	case "which sponsors haven't paid yet":
		transactionLines, err := egotivities.TransactionLines(bot.eactivities, bot.centreNum, egotivities.CurrentYear())
		if err != nil {
			return "", err
		}
		var pendingPayment []string
		pendingPaymentMoney := &big.Float{}
		var pendingInvoice []string
		pendingInvoiceMoney := &big.Float{}
		r := regexp.MustCompile(`^[^(]+\((.*)\)$`)
		for _, l := range transactionLines {
			if l.Account.Code != "550" || l.Activity.Code != "00" || !strings.HasPrefix(l.Document, "SI ") {
				continue
			}
			nameBits := r.FindStringSubmatch(l.Description)
			switch {
			case l.Outstanding:
				pendingPayment = append(pendingPayment, fmt.Sprintf("• %s - %s", nameBits[1], formatMoney(&l.Amount.Float)))
				pendingPaymentMoney.Add(pendingPaymentMoney, &l.Amount.Float)
			case l.Pending:
				pendingInvoice = append(pendingInvoice, fmt.Sprintf("• %s - %s", nameBits[1], formatMoney(&l.Amount.Float)))
				pendingInvoiceMoney.Add(pendingInvoiceMoney, &l.Amount.Float)
			}
		}
		ppstr := strings.Join(pendingPayment, "\n")
		pistr := strings.Join(pendingInvoice, "\n")
		ppsponsorstr := "sponsors"
		if len(pendingPayment) == 1 {
			ppsponsorstr = "sponsor"
		}
		piinvoicestr := "invoices"
		if len(pendingInvoice) == 1 {
			piinvoicestr = "invoice"
		}
		switch {
		case len(pendingPayment) > 0 && len(pendingInvoice) > 0:
			return fmt.Sprintf("We're waiting for *%d %s to pay* for a total of %s:\n%s\nand for *%d %s to be approved* for a total of %s:\n%s", len(pendingPayment), ppsponsorstr, formatMoney(pendingPaymentMoney), ppstr, len(pendingInvoice), piinvoicestr, formatMoney(pendingInvoiceMoney), pistr), nil
		case len(pendingPayment) > 0:
			return fmt.Sprintf("We're waiting for *%d %s to pay* for a total of %s:\n%s", len(pendingPayment), ppsponsorstr, formatMoney(pendingPaymentMoney), ppstr), nil
		case len(pendingInvoice) > 0:
			return fmt.Sprintf("We're waiting for *%d %s to be approved* for a total of %s:\n%s", len(pendingInvoice), piinvoicestr, formatMoney(pendingInvoiceMoney), pistr), nil
		default:
			return fmt.Sprintf("We're not waiting for any sponsors or invoices. Hooray! 🎉🎉🎉"), nil
		}
	}
	return "", nil
}

// New creates a new Bot.
func New(opts BotOptions) (*Bot, error) {
	api := slack.New(opts.Token)
	client := egotivities.NewClient(opts.EActivitiesKey)
	return &Bot{
		api:         api,
		eactivities: client,
		centreNum:   opts.EActivitiesCentre,
	}, nil
}
