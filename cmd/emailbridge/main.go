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
	"time"
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
			Value:       "config.xml",
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
				if err = emailbridge.LoadConfig(configFile, &config); err == nil {
					if _, err = emailbridge.NewEmailBridge(&config, http.DefaultServeMux); err == nil {

						serverAddr := fmt.Sprintf("%v:%v", config.Server, config.Port)

						logrus.Infof("Start server at http://%v", serverAddr)
						err = http.ListenAndServe(serverAddr, nil)
					}
				}
				return
			},
		}, {
			Name:  "sendExample",
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

				var config emailbridge.Config
				if err = emailbridge.LoadConfig(configFile, &config); err == nil {
					var bridge *emailbridge.HttpEmailBridge
					if bridge, err = emailbridge.NewEmailBridge(&config, nil); err == nil {

						var message *email.Message
						if message, err = bridge.BuildEmail(receiverEmail, "Test "+time.Now().String(),
							"This is markdown body"); err == nil {
							err = bridge.Send(message)
						}
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
					Usage:       "Config target file name to generate",
					Value:       "config.yml",
					Destination: &targetFile,
				},
			},
			Action: func(c *cli.Context) (err error) {
				err = emailbridge.WriteConfig(targetFile, emailbridge.BuildDefault())
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
