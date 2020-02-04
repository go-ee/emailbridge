package emailbridge

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"github.com/go-ee/utils/net/smtp"
	"github.com/sirupsen/logrus"
	"mime/quotedprintable"
	"strings"
)

const (
	SMTPHost         = "smtp.gmail.com"
	SMTPHostWithPort = "smtp.gmail.com:587"
)

type EmailSender struct {
	User     string
	Password string
}

func NewEmailSender(Username, Password string) *EmailSender {
	return &EmailSender{Username, Password}
}

func (o EmailSender) SendMail(Dest []string, Subject, message string) (err error) {
	msg := fmt.Sprintf("From: %v\nTo: %v\nSubject: %v\n%v",
		o.User, strings.Join(Dest, ","), Subject, message)

	logrus.Debugf("SendMail, %v, %v", Dest, Subject)

	if err = smtp.SendMail(SMTPHostWithPort,
		smtp.PlainAuth("", o.User, o.Password, SMTPHost),
		o.User, Dest, []byte(msg), &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         SMTPHost,
		}); err != nil {

		logrus.Warnf("SendMail, err=%v, %v, %v", err, Dest, Subject)
	}
	return
}

func (o EmailSender) BuildEmail(dest []string, contentType, subject, bodyMessage string) string {

	header := make(map[string]string)
	header["From"] = o.User

	receipient := ""

	for _, user := range dest {
		receipient = receipient + user
	}

	header["To"] = receipient
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = fmt.Sprintf("%s; charset=\"utf-8\"", contentType)
	header["Content-Transfer-Encoding"] = "quoted-printable"
	header["Content-Disposition"] = "inline"

	message := ""

	for key, value := range header {
		message += fmt.Sprintf("%s: %s\r\n", key, value)
	}

	var encodedMessage bytes.Buffer

	finalMessage := quotedprintable.NewWriter(&encodedMessage)
	finalMessage.Write([]byte(bodyMessage))
	finalMessage.Close()

	message += "\r\n" + encodedMessage.String()

	return message
}

func (o *EmailSender) BuildHTMLEmail(dest []string, subject, bodyMessage string) string {

	return o.BuildEmail(dest, "text/html", subject, bodyMessage)
}

func (o *EmailSender) BuildPlainEmail(dest []string, subject, bodyMessage string) string {

	return o.BuildEmail(dest, "text/plain", subject, bodyMessage)
}
