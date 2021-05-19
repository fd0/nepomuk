package notify

import (
	"fmt"
	"os"
	"strings"

	"github.com/fd0/nepomuk/database"
	"github.com/gregdel/pushover"
	"github.com/sirupsen/logrus"
)

var (
	token      = os.Getenv("NEPOMUK_PUSHOVER_TOKEN")
	recipients = os.Getenv("NEPOMUK_PUSHOVER_RECIPIENTS")
)

func Notify(logger logrus.FieldLogger, file database.File) {
	log := logger.WithField("component", "database-watcher")
	if token == "" {
		log.Warn("no pushover token found, skipping notification")

		return
	}

	if recipients == "" {
		log.Warn("no recipients found, skipping notification")

		return
	}

	// Create a new pushover app with a token
	app := pushover.New(token)

	// Create the message to send
	message := pushover.NewMessageWithTitle(
		fmt.Sprintf("Archiver found new file %v for correspondent %v", file.Filename, file.Correspondent),
		"Archive: new file",
	)

	// Create a new recipient
	for _, r := range strings.Split(recipients, ",") {
		recipient := pushover.NewRecipient(r)

		// Send the message to the recipient
		response, err := app.SendMessage(message, recipient)
		if err != nil {
			fmt.Fprintf(os.Stderr, "unable to send message: %v\n", err)
		}

		// Print the response if you want
		log.Printf("response from pushover: %v", response)
	}
}
