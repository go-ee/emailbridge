package main

import (
	"fmt"
	"github.com/go-ee/emailbridge"
	"github.com/go-ee/utils/email"
	"github.com/go-ee/utils/lg"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"net/http"
	"os"
)

func main() {
	app := cli.NewApp()
	app.Usage = "Email Bridge CLI"
	app.Version = "1.0"

	var serverAddress, emailAddress, receiverEmail, smtpLogin, smtpPassword, smtpHost string
	var serverPort, smtpPort int
	var debug bool
	var pathStorage, pathStatic, encryptPassphrase string

	lg.LogrusTimeAsTimestampFormatter()

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
			Name:        "email",
			Usage:       "Email address of sender",
			Required:    true,
			Destination: &emailAddress,
		}, &cli.StringFlag{
			Name:        "smtpLogin",
			Usage:       "SMTP login",
			Required:    true,
			Destination: &smtpLogin,
		}, &cli.StringFlag{
			Name:        "smtpPassword",
			Usage:       "SMTP password",
			Required:    true,
			Destination: &smtpPassword,
		}, &cli.StringFlag{
			Name:        "smtpHost",
			Usage:       "SMTP server host",
			Value:       "smtp.gmail.com",
			Destination: &smtpHost,
		}, &cli.IntFlag{
			Name:        "smtpPort",
			Usage:       "SMTP server port",
			Value:       587,
			Destination: &smtpPort,
		},
	}

	app.Commands = []*cli.Command{
		{
			Name:  "startBridge",
			Usage: "Start HTTP to EMAIL Bridge",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:        "serverAddress",
					Required:    true,
					Destination: &serverAddress,
					Value:       "",
					Usage:       "HTTP Server serverAddress",
				}, &cli.IntFlag{
					Name:        "serverPort",
					Required:    true,
					Destination: &serverPort,
					Value:       8080,
					Usage:       "HTTP Server serverPort",
				}, &cli.StringFlag{
					Name:        "pathStatic",
					Required:    true,
					Value:       "static",
					Destination: &pathStatic,
				}, &cli.StringFlag{
					Name:        "pathStorage",
					Required:    true,
					Value:       "storage",
					Destination: &pathStorage,
				}, &cli.StringFlag{
					Name:        "encryptPassphrase",
					Destination: &encryptPassphrase,
				},
			},
			Action: func(c *cli.Context) (err error) {
				if encryptPassphrase == "" {
					encryptPassphrase = smtpPassword
				}

				var bridge *emailbridge.HttpEmailBridge
				if bridge, err = emailbridge.NewEmailBridge(emailAddress, smtpLogin, smtpPassword, smtpHost, smtpPort,
					pathStorage, pathStatic, encryptPassphrase); err == nil {

					wireRoutes(bridge)

					serverAddr := fmt.Sprintf("%v:%v", serverAddress, serverPort)

					logrus.Infof("Start server at %v", serverAddr)
					err = http.ListenAndServe(serverAddr, nil)
				}
				return
			},
		}, {
			Name:  "sendExampleEmail",
			Usage: "Send example email",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:        "receiverEmail",
					Usage:       "Receiver Email serverAddress",
					Required:    true,
					Destination: &receiverEmail,
				},
			},
			Action: func(c *cli.Context) (err error) {

				sender := email.NewSender(emailAddress, smtpLogin, smtpPassword, smtpHost, smtpPort)

				receiver := []string{receiverEmail}

				subject := "Test Email from EmailBridge"
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
				if bodyMessage, err := sender.BuildEmailHTML(message); err != nil {
					err = sender.Send(receiver, subject, bodyMessage)
				}
				return
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		if markdown, err := app.ToMarkdown(); err == nil {
			logrus.Infof("\n%v", markdown)
		} else {
			logrus.Infof("%v", err)
		}

		logrus.WithFields(logrus.Fields{"err": err}).Warn("exit because of error.")
	}
}

func wireRoutes(bridge *emailbridge.HttpEmailBridge) {

	http.HandleFunc("/favicon.ico", bridge.FaviconHandler)
	http.HandleFunc("/generate", bridge.GenerateEncryptedCode)
	http.HandleFunc("/sendEmail", bridge.SendEmail)
	http.HandleFunc("/sendEmailByEncryptedCode", bridge.SendEmailByEncryptedCode)
	http.HandleFunc("/", bridge.GenerateEncryptedCode)

	return
}
