package main

import (
	"github.com/go-ee/emailbridge"
	"os"
	"s2r/shared"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Usage = "Email Bridge CLI"
	app.Version = "1.0"

	var senderEmail, senderPassword string
	var pathStatic, encryptPassphrase string

	var debug bool
	var port int

	shared.LogrusTimeAsTimestampFormatter()

	app.Before = func(c *cli.Context) (err error) {
		if debug {
			logrus.SetLevel(logrus.DebugLevel)
		}
		logrus.Debugf("execute %v", c.Command.Name)
		return
	}

	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:        "debug",
			Destination: &debug,
			Usage:       "Enable debug log level",
		}, &cli.StringFlag{
			Name:        "senderEmail",
			Usage:       "Sender Email for authentication and FROM field",
			Required:    true,
			Destination: &senderEmail,
		}, &cli.StringFlag{
			Name:        "senderPassword",
			Usage:       "Sender password for authentication ",
			Required:    true,
			Destination: &senderPassword,
		},
	}

	app.Commands = []*cli.Command{
		{
			Name:  "startBridge",
			Usage: "Start HTTP to EMAIL Bridge",
			Flags: []cli.Flag{
				&cli.IntFlag{
					Name:        "port",
					Destination: &port,
					Value:       8080,
					Usage:       "HTTP Server port",
				}, &cli.StringFlag{
					Name:        "pathStatic",
					Value:       "static",
					Destination: &pathStatic,
				}, &cli.StringFlag{
					Name:        "encryptPassphrase",
					Destination: &encryptPassphrase,
				},
			},
			Action: func(c *cli.Context) (err error) {
				if encryptPassphrase == "" {
					encryptPassphrase = senderPassword
				}

				var bridge *emailbridge.HttpEmailBridge
				if bridge, err = emailbridge.NewEmailBridge(senderEmail, senderPassword,
					port, pathStatic, encryptPassphrase); err == nil {

					err = bridge.Start()
				}
				return
			},
		}, {
			Name:  "sendExampleMessage",
			Usage: "Send example message",
			Flags: []cli.Flag{
			},
			Action: func(c *cli.Context) (err error) {

				sender := emailbridge.NewEmailSender(senderEmail, senderPassword)

				//The receiver needs to be in slice as the receive supports multiple receiver
				Receiver := []string{"otschen.prosto@gmail.com"}

				Subject := "Testing HTLML Email from golang"
				message := `
	<!DOCTYPE HTML PULBLIC "-//W3C//DTD HTML 4.01 Transitional//EN">
	<html>
	<head>
	<meta http-equiv="content-type" content="text/html"; charset=ISO-8859-1">
	</head>
	<body>This is the body<br>
	<div class="moz-signature"><i><br>
	<br>
	Regards<br>
	Alex<br>
	<i></div>
	</body>
	</html>
	`
				bodyMessage := sender.BuildHTMLEmail(Receiver, Subject, message)

				sender.SendMail(Receiver, Subject, bodyMessage)

				return
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.WithFields(logrus.Fields{"err": err}).Warn("exit because of error.")
	}
}
