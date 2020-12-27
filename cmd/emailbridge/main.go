package main

import (
	"fmt"
	"github.com/go-ee/emailbridge"
	"github.com/go-ee/utils/email"
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

	var configFile, receiverEmail, targetFile string
	var debug bool

	lg.LogrusTimeAsTimestampFormatter()

	app.Before = func(c *cli.Context) (err error) {
		if debug {
			logrus.SetLevel(logrus.DebugLevel)
		}
		logrus.Debugf("execute %v", c.Command.Name)
		return
	}

	commonFlags := []cli.Flag{
		&cli.BoolFlag{
			Name:        "debug",
			Aliases:     []string{"d"},
			Destination: &debug,
			Usage:       "Enable debug log level",
		}, &cli.StringFlag{
			Name:        "config",
			Aliases:     []string{"c"},
			Usage:       "EmailBridge config file",
			Value:       "config.yml",
			Destination: &configFile,
		},
	}

	app.Commands = []*cli.Command{
		{
			Name:  "startBridge",
			Usage: "Start HTTP to EMAIL Bridge",
			Flags: commonFlags,
			Action: func(c *cli.Context) (err error) {

				var config emailbridge.Config
				if err = emailbridge.ConfigLoad(configFile, &config); err == nil {
					if _, err = emailbridge.NewEmailBridge(&config, http.DefaultServeMux); err == nil {

						serverAddr := fmt.Sprintf("%v:%v", config.Server, config.Port)

						linkHost := config.Server
						if linkHost == "" || linkHost == "0.0.0.0" {
							linkHost = "127.0.0.1"
						}

						logrus.Infof("Start server at http://%v:%v", linkHost, config.Port)

						err = http.ListenAndServe(serverAddr, nil)
					}
				}
				return
			},
		}, {
			Name:  "sendExample",
			Usage: "Send example email",
			Flags: append(commonFlags,
				&cli.StringFlag{
					Name:        "receiverEmail",
					Aliases:     []string{"r"},
					Usage:       "Receiver Email serverAddress",
					Required:    true,
					Destination: &receiverEmail,
				}),
			Action: func(c *cli.Context) (err error) {

				var config emailbridge.Config
				if err = emailbridge.ConfigLoad(configFile, &config); err == nil {
					var bridge *emailbridge.HttpEmailBridge
					if bridge, err = emailbridge.NewEmailBridge(&config, nil); err == nil {
						err = bridge.Send(
							&email.EmailData{
								To:       []string{receiverEmail},
								Name:     "TestName",
								Subject:  "TestSubject",
								Url:      "TestUrl",
								Markdown: "This is markdown body",
							})
					}
				}
				return
			},
		},
		{
			Name:  "config",
			Usage: "Generate default config file",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:        "target",
					Aliases:     []string{"t"},
					Usage:       "EngineConfig target file name to generate",
					Value:       "config.yml",
					Destination: &targetFile,
				},
			},
			Action: func(c *cli.Context) (err error) {
				err = emailbridge.BuildDefault().WriteConfig(targetFile)
				return
			},
		},
		{
			Name:  "markdown",
			Usage: "Generate markdown help file",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:        "target",
					Aliases:     []string{"t"},
					Usage:       "Markdown target file name to generate",
					Value:       "email-bridge.md",
					Destination: &targetFile,
				},
			},
			Action: func(c *cli.Context) (err error) {
				var markdown string
				if markdown, err = app.ToMarkdown(); err == nil {
					err = ioutil.WriteFile(targetFile, []byte(markdown), 0)
				}
				return
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.WithFields(logrus.Fields{"err": err}).Warn("exit because of error.")
	}
}
