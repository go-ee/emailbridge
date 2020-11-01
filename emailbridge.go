package emailbridge

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-ee/utils/encrypt"
	"github.com/go-ee/utils/net"
	"github.com/sirupsen/logrus"
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

func NewEmailBridge(
	senderEmail, senderPassword string, smtpHost string, smtpPort int,
	port int, pathStorage string, pathStatic string,
	encryptPassphrase string) (ret *HttpEmailBridge, err error) {

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
		EmailSender:   NewEmailSender(senderEmail, senderPassword, smtpHost, smtpPort),
		Encryptor:     encryptor,
		Port:          port,
		PathStorage:   pathStorage,
		PathStatic:    pathStatic,
		emailDataTmpl: emailDataFormTemplate,
	}
	return
}

type HttpEmailBridge struct {
	*EmailSender
	*encrypt.Encryptor
	Port        int
	PathStatic  string
	PathStorage string

	emailDataTmpl *template.Template
	storeEmails   bool
}

func (o *HttpEmailBridge) Start() (err error) {

	if err = o.checkAndCreateStorage(); err != nil {
		return
	}

	if err = o.checkAndCreateStatic(); err != nil {
		return
	}

	http.HandleFunc("/favicon.ico", o.faviconHandler)
	http.HandleFunc("/generate", o.generateLink)
	http.HandleFunc("/sendemail", o.sendEmail)
	http.HandleFunc("/", o.emailData)
	serverAddr := fmt.Sprintf(":%v", o.Port)

	logrus.Infof("Start server at %v", serverAddr)
	err = http.ListenAndServe(serverAddr, nil)
	return
}

func (o *HttpEmailBridge) checkAndCreateStorage() (err error) {
	o.storeEmails = false
	if o.PathStorage != "" {
		if err = os.MkdirAll(o.PathStorage, 0755); err == nil {
			o.storeEmails = true
			logrus.Infof("use the storage path: %v", o.PathStorage)
		} else {
			logrus.Infof("can't create the storage path '%v': %v", o.PathStorage, err)
		}
	}
	return
}

func (o *HttpEmailBridge) checkAndCreateStatic() (err error) {
	if o.PathStatic != "" {
		if err = os.MkdirAll(o.PathStatic, 0755); err == nil {
			logrus.Infof("use the static path: %v", o.PathStatic)
		} else {
			err = errors.New("path for static files not defined")
		}
	}
	return
}

func (o *HttpEmailBridge) emailData(w http.ResponseWriter, r *http.Request) {
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

func (o *HttpEmailBridge) generateLink(w http.ResponseWriter, r *http.Request) {
	emailData := decodeEmailDataParams(r)

	if len(emailData.To) == 0 || emailData.Name == "" || emailData.Subject == "" {
		var emailDataForm bytes.Buffer
		if err := o.emailDataTmpl.Execute(&emailDataForm, emailData); err != nil {
			statusBadRequest(w, err.Error())
		} else {
			statusBadRequest(w, emailDataForm.String())
		}
	} else {
		logrus.Debugf("generateLink, %v, %v", emailData.To, emailData.Subject)

		if data, err := o.EncryptInstance(emailData); err == nil {
			statusOk(w, fmt.Sprintf("%v?%v=%v", emailData.Url, paramEmailCode, hex.EncodeToString(data)))
		} else {
			statusBadRequest(w, err.Error())
		}
	}
	return
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

func (o *HttpEmailBridge) sendEmail(w http.ResponseWriter, r *http.Request) {
	var emailData *EmailData
	if emailData = o.decryptEmailData(w, r); emailData == nil {
		return
	}

	logrus.Debugf("sendEmail, %v, %v", emailData.To, emailData.Subject)

	var emailBody string
	emailBody = net.GetQueryOrFormValue(paramEmailBody, r)

	htmlMessage := o.BuildHTMLEmail(emailData.To, emailData.Subject, emailBody)

	o.storeEmail(r, &htmlMessage, emailData)

	if err := o.SendMail(emailData.To, emailData.Subject, htmlMessage); err == nil {
		statusOk(w, "email sent successfully.")
	} else {
		statusBadRequest(w, err.Error())
	}
}

func (o *HttpEmailBridge) storeEmail(r *http.Request, htmlMessage *string, emailData *EmailData) {
	if o.storeEmails {
		fileData := []byte(fmt.Sprintf("request:\n%v\n\nmessage:\n%v\n", net.FormatRequestFrom(r), htmlMessage))
		filePath := filepath.Clean(fmt.Sprintf("%v/%v_%v_%v.txt",
			o.PathStorage, strings.Join(emailData.To, "_"), emailData.Subject, time.Now()))
		if fileErr := ioutil.WriteFile(filePath, fileData, 0644); fileErr != nil {
			logrus.Warnf("can't write '%v', %v", filePath, fileErr)
		} else {
			logrus.Debugf("written '%v', bytes=%v", filePath, len(fileData))
		}
	}
}

func statusOk(w http.ResponseWriter, msg string) {
	logrus.Debug("statusOk, %v", msg)
	net.EnableCors(w)
	if _, resErr := w.Write([]byte(msg)); resErr != nil {
		logrus.Debug("error writing response %v", resErr)
	}
}

func statusBadRequest(w http.ResponseWriter, msg string) {
	logrus.Warnf("statusBadRequest, %v", msg)

	net.EnableCors(w)
	w.WriteHeader(http.StatusBadRequest)
	if _, resErr := w.Write([]byte(msg)); resErr != nil {
		logrus.Debug("error writing response %v", resErr)
	}
}

func (o *HttpEmailBridge) faviconHandler(w http.ResponseWriter, r *http.Request) {
	favicon := fmt.Sprintf("%v/favicon.ico", o.PathStatic)
	http.ServeFile(w, r, favicon)
}