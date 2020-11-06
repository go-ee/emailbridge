package main

import (
	"fmt"
	"github.com/go-ee/emailbridge"
	"github.com/go-ee/utils/lg"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"io/ioutil"
	"net/http"
	"os"
)

func main() {
	app := cli.NewApp()
	app.Usage = "Email Bridge CLI"
	app.Version = "1.0"

	var address, emailAddress, receiverEmail, smtpLogin, smtpPassword, smtpHost, targetFile string
	var port, smtpPort int
	var verbose bool
	var pathStorage, pathTemplates, encryptPassphrase string

	lg.LogrusTimeAsTimestampFormatter()

	app.Before = func(c *cli.Context) (err error) {
		if verbose {
			logrus.SetLevel(logrus.DebugLevel)
		}
		logrus.Debugf("execute %v", c.Command.Name)
		return
	}

	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:        "verbose",
			Destination: &verbose,
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
					Name:        "address",
					Aliases:     []string{"a"},
					Usage:       "Host for the HTTP server",
					Value:       "0.0.0.0",
					Destination: &address,
				}, &cli.IntFlag{
					Name:        "port",
					Aliases:     []string{"p"},
					Usage:       "port for the HTTP server",
					Value:       8080,
					Destination: &port,
				}, &cli.StringFlag{
					Name:        "pathTemplates",
					Required:    true,
					Value:       "templates",
					Destination: &pathTemplates,
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
					pathStorage, pathTemplates, encryptPassphrase); err == nil {

					wireRoutes(bridge)

					serverAddr := fmt.Sprintf("%v:%v", address, port)

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

				var bridge *emailbridge.HttpEmailBridge
				if bridge, err = emailbridge.NewEmailBridge(emailAddress, smtpLogin, smtpPassword, smtpHost, smtpPort,
					pathStorage, pathTemplates, encryptPassphrase); err != nil {
					return
				}

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
				if bodyMessage, err := bridge.BuildEmailHTML(message); err != nil {
					err = bridge.Send(receiverEmail, subject, bodyMessage, bodyMessage)
				}
				return
			},
		},
		{
			Name:  "markdown",
			Usage: "Generate markdown help file",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:        "target",
					Aliases:     []string{"-t"},
					Usage:       "Markdown target file name to generate",
					Required:    true,
					Value:       "email-bridge.md",
					Destination: &targetFile,
				},
			},
			Action: func(c *cli.Context) (err error) {
				if markdown, err := app.ToMarkdown(); err == nil {
					err = ioutil.WriteFile(targetFile, []byte(markdown), 0)
				} else {
					logrus.Infof("%v", err)
				}
				return
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.WithFields(logrus.Fields{"err": err}).Warn("exit because of error.")
	}
}

func wireRoutes(bridge *emailbridge.HttpEmailBridge) {

	http.HandleFunc("/email-code/code", bridge.GenerateEncryptedCode)
	http.HandleFunc("/email-code/link", bridge.GenerateEncryptedCodeWithLink)
	http.HandleFunc("/email/send", bridge.SendEmail)
	http.HandleFunc("/email/send-code", bridge.SendEmailByEncryptedCode)
	http.HandleFunc("/favicon.ico", bridge.FaviconHandler)

	return
}
