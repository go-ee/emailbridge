package emailbridge

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-ee/utils/email"
	"github.com/go-ee/utils/encrypt"
	"github.com/go-ee/utils/net"
	"github.com/sirupsen/logrus"
	"gopkg.in/alexcesaro/quotedprintable.v3"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	paramTo      = "to"
	paramName    = "name"
	paramSubject = "subject"
	paramUrl     = "url"

	paramEmailCode = "emailCode"
	paramEmailBody = "emailBody"

	emailDataFormText = `
	<!DOCTYPE HTML PULBLIC "-//W3C//DTD HTML 4.01 Transitional//EN">
	<html>
		<head>
			<meta http-equiv="content-type" content="text/html"; charset="ISO-8859-1">
		</head>
		<body>
			<form>
			  <label for="to">Email address:</label><br>
			  <input type="text" id="to" name="to" value="{{ StringsJoin .To "," }}"/><br>
			  <label for="name">Name:</label><br>
			  <input type="text" id="name" name="name" value="{{ .Name }}"/><br>
			  <label for="subject">subject:</label><br>
			  <input type="text" id="subject" name="subject" value="{{ .Subject }}"/><br>
			  <label for="subject">url:</label><br>
			  <input type="text" id="url" name="url" value="{{ .Url }}"/><br>
			  <input type="submit"/>	
			</form>
		</body>
	</html>
	`
)

type EmailData struct {
	To        []string
	Name      string
	Subject   string
	Url       string
	CreatedAt time.Time
}

func (o *EmailData) ToString() string {
	return strings.Join(o.To, ",")
}

func NewEmailBridge(
	emailAddress, smtpLogin, smtpPassword, smtpHost string, smtpPort int,
	pathStorage string, pathStatic string, encryptPassphrase string) (ret *HttpEmailBridge, err error) {

	var encryptor *encrypt.Encryptor

	if encryptor, err = encrypt.NewEncryptor(encryptPassphrase); err != nil {
		return
	}

	var emailDataFormTemplate *template.Template

	if emailDataFormTemplate, err = template.New("test").
		Funcs(template.FuncMap{"StringsJoin": strings.Join}).Parse(emailDataFormText); err != nil {
		return
	}

	ret = &HttpEmailBridge{
		Sender: &email.Sender{Server: smtpHost, Port: smtpPort, SenderEmail: emailAddress,
			SenderIdentity: emailAddress, SMTPUser: smtpLogin, SMTPPassword: smtpPassword},
		Encryptor:     encryptor,
		pathStorage:   pathStorage,
		pathStatic:    pathStatic,
		emailDataTmpl: emailDataFormTemplate,
	}

	if err = ret.checkAndCreateStorage(); err != nil {
		return
	}

	if err = ret.checkAndCreateStatic(); err != nil {
		return
	}

	return
}

type HttpEmailBridge struct {
	*email.Sender
	*encrypt.Encryptor
	pathStatic    string
	pathStorage   string
	emailDataTmpl *template.Template
	storeEmails   bool
}

func (o *HttpEmailBridge) SendEmail(w http.ResponseWriter, r *http.Request) {
	emailData := decodeEmailDataParams(r)

	if len(emailData.To) == 0 || emailData.Name == "" || emailData.Subject == "" {
		var emailDataForm bytes.Buffer
		if err := o.emailDataTmpl.Execute(&emailDataForm, emailData); err != nil {
			statusBadRequest(w, err.Error())
		} else {
			statusBadRequest(w, emailDataForm.String())
		}
	} else {
		o.sendEmailByEmailData(emailData, w, r)
	}
	return
}

func (o *HttpEmailBridge) GenerateEncryptedCode(w http.ResponseWriter, r *http.Request) {
	var emailData *EmailData
	if emailData = o.decryptEmailData(w, r); emailData == nil {
		return
	}
	if jsonData, err := json.Marshal(emailData); err == nil {
		statusOk(w, string(jsonData))
	} else {
		statusBadRequest(w, err.Error())
	}
}

func (o *HttpEmailBridge) GenerateEncryptedCodeWithLink(w http.ResponseWriter, r *http.Request) {
	emailData := decodeEmailDataParams(r)

	if len(emailData.To) == 0 || emailData.Name == "" || emailData.Subject == "" {
		var emailDataForm bytes.Buffer
		if err := o.emailDataTmpl.Execute(&emailDataForm, emailData); err != nil {
			statusBadRequest(w, err.Error())
		} else {
			statusBadRequest(w, emailDataForm.String())
		}
	} else {
		logrus.Debugf("GenerateEncryptedCode, %v, %v", emailData.To, emailData.Subject)

		if data, err := o.EncryptInstance(emailData); err == nil {
			statusOk(w, fmt.Sprintf("%v?%v=%v", emailData.Url, paramEmailCode, hex.EncodeToString(data)))
		} else {
			statusBadRequest(w, err.Error())
		}
	}
	return
}

func (o *HttpEmailBridge) FaviconHandler(w http.ResponseWriter, r *http.Request) {
	favicon := fmt.Sprintf("%v/favicon.ico", o.pathStatic)
	http.ServeFile(w, r, favicon)
}

func (o *HttpEmailBridge) SendEmailByEncryptedCode(w http.ResponseWriter, r *http.Request) {
	var emailData *EmailData
	if emailData = o.decryptEmailData(w, r); emailData == nil {
		return
	}

	o.sendEmailByEmailData(emailData, w, r)
}

func (o *HttpEmailBridge) sendEmailByEmailData(emailData *EmailData, w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("sendEmailByEmailData, %v, %v", emailData.To, emailData.Subject)

	var emailBody string
	emailBody = net.GetQueryOrFormValue(paramEmailBody, r)

	htmlMessage, err := o.BuildEmailHTML(emailBody)
	if err != nil {
		statusBadRequest(w, err.Error())
		return
	}

	o.storeEmail(r, &htmlMessage, emailData)

	if err := o.Send(strings.Join(emailData.To, ","), emailData.Subject, htmlMessage, htmlMessage); err == nil {
		statusOk(w, "email sent successfully.")
	} else {
		statusBadRequest(w, err.Error())
	}
}

func decodeEmailDataParams(r *http.Request) (ret *EmailData) {
	ret = &EmailData{
		To:        strings.Split(net.GetQueryOrFormValue(paramTo, r), ","),
		Name:      net.GetQueryOrFormValue(paramName, r),
		Subject:   net.GetQueryOrFormValue(paramSubject, r),
		Url:       net.GetQueryOrFormValue(paramUrl, r),
		CreatedAt: time.Now()}
	return
}

func (o *HttpEmailBridge) decryptEmailData(w http.ResponseWriter, r *http.Request) (ret *EmailData) {
	var encrypted string
	if encrypted = net.GetQueryOrFormValue(paramEmailCode, r); encrypted == "" {
		parameterNotProvided(w, paramEmailCode)
		return
	}

	if data, err := hex.DecodeString(encrypted); err == nil {
		if decryptErr := o.DecryptInstance(&ret, data); decryptErr != nil {
			statusBadRequest(w, decryptErr.Error())
		}
	} else {
		statusBadRequest(w, err.Error())
	}
	return
}

func parameterNotProvided(w http.ResponseWriter, param string) {
	statusBadRequest(w, fmt.Sprintf("'%v' parameter is not provided", param))
}

func (o *HttpEmailBridge) storeEmail(r *http.Request, htmlMessage *string, emailData *EmailData) {
	if o.storeEmails {
		fileData := []byte(fmt.Sprintf("request:\n%v\n\nmessage:\n%v\n", net.FormatRequestFrom(r), htmlMessage))
		filePath := filepath.Clean(fmt.Sprintf("%v/%v_%v_%v.txt",
			o.pathStorage, strings.Join(emailData.To, "_"), emailData.Subject, time.Now()))
		if fileErr := ioutil.WriteFile(filePath, fileData, 0644); fileErr != nil {
			logrus.Warnf("can't write '%v', %v", filePath, fileErr)
		} else {
			logrus.Debugf("written '%v', bytes=%v", filePath, len(fileData))
		}
	}
}

func statusOk(w http.ResponseWriter, msg string) {
	logrus.Debug("statusOk, %v", msg)
	// net.CorsAllowAll(w)
	if _, resErr := w.Write([]byte(msg)); resErr != nil {
		logrus.Debug("error writing response %v", resErr)
	}
}

func statusBadRequest(w http.ResponseWriter, msg string) {
	logrus.Warnf("statusBadRequest, %v", msg)

	// net.CorsAllowAll(w)
	w.WriteHeader(http.StatusBadRequest)
	if _, resErr := w.Write([]byte(msg)); resErr != nil {
		logrus.Debug("error writing response %v", resErr)
	}
}

func (o *HttpEmailBridge) checkAndCreateStorage() (err error) {
	o.storeEmails = false
	if o.pathStorage != "" {
		if err = os.MkdirAll(o.pathStorage, 0755); err == nil {
			o.storeEmails = true
			logrus.Infof("use the storage path: %v", o.pathStorage)
		} else {
			logrus.Infof("can't create the storage path '%v': %v", o.pathStorage, err)
		}
	}
	return
}

func (o *HttpEmailBridge) checkAndCreateStatic() (err error) {
	if o.pathStatic != "" {
		if err = os.MkdirAll(o.pathStatic, 0755); err == nil {
			logrus.Infof("use the static path: %v", o.pathStatic)
		} else {
			err = errors.New("path for static files not defined")
		}
	}
	return
}

func (o HttpEmailBridge) BuildEmail(contentType, body string) (ret string, err error) {

	header := make(map[string]string)

	header["Date"] = time.Now().Format(time.RFC1123Z)
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = fmt.Sprintf("%s; charset=\"utf-8\"", contentType)
	header["Content-Transfer-Encoding"] = "quoted-printable"
	header["Content-Disposition"] = "inline"

	for key, value := range header {
		ret += fmt.Sprintf("%s: %s\r\n", key, value)
	}

	var encodedMessage bytes.Buffer

	finalMessage := quotedprintable.NewWriter(&encodedMessage)
	if _, err = finalMessage.Write([]byte(body)); err == nil {
		return
	}
	if err = finalMessage.Close(); err == nil {
		return
	}
	ret += "\r\n" + encodedMessage.String()
	return
}

func (o *HttpEmailBridge) BuildEmailHTML(body string) (ret string, err error) {
	ret, err = o.BuildEmail("text/html", body)
	return
}

func (o *HttpEmailBridge) BuildEmailPlain(body string) (ret string, err error) {
	ret, err = o.BuildEmail("text/plain", body)
	return
}
