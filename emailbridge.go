package emailbridge

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-ee/utils/email"
	"github.com/go-ee/utils/encrypt"
	"github.com/go-ee/utils/net"
	"github.com/matcornic/hermes/v2"
	"github.com/sirupsen/logrus"
)

const (
	paramTo      = "to"
	paramName    = "name"
	paramSubject = "subject"
	paramUrl     = "url"

	paramEmailCode = "emailCode"
	paramMarkdown  = "Markdown"

	emailDataFormText = `
	<!DOCTYPE HTML PULBLIC "-//W3C//DTD HTML 4.01 Transitional//EN">
	<html>
		<head>
			<meta http-equiv="content-type" content="text/html"; charset="ISO-8859-1">
		</head>
		<body>
			<form>
			  <label for="To">Email address:</label><br>
			  <input type="text" id="To" name="to" value="{{ StringsJoin .To "," }}"/><br>
			  <label for="name">Name:</label><br>
			  <input type="text" id="Name" name="Name" value="{{ .Name }}"/><br>
			  <label for="Subject">Subject:</label><br>
			  <input type="text" id="Subject" name="Subject" value="{{ .Subject }}"/><br>
			  <br> 
			  <label for="Url">url:</label><br>
			  <input type="text" id="Url" name="Url" value="{{ .Url }}"/><br>
			  <br> 
			  <label for="Markdown">Markdown:</label><br>
			  <input type="text" id="Markdown" name="Markdown" value="{{ .Markdown }}"/><br>		
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
	Markdown  string
	CreatedAt time.Time
}

func (o *EmailData) ToAsString() string {
	return strings.Join(o.To, ",")
}

type HttpEmailBridge struct {
	*hermes.Hermes
	*hermes.Body
	*email.Sender
	*encrypt.Encryptor
	root          string
	pathStorage   string
	emailDataTmpl *template.Template
	storeEmails   bool
}

func NewEmailBridge(config *Config, serveMux *http.ServeMux) (ret *HttpEmailBridge, err error) {

	var encryptor *encrypt.Encryptor

	if encryptor, err = encrypt.NewEncryptor(config.EncryptPassphrase); err != nil {
		return
	}

	var emailDataFormTemplate *template.Template

	if emailDataFormTemplate, err = template.New("test").
		Funcs(template.FuncMap{"StringsJoin": strings.Join}).Parse(emailDataFormText); err != nil {
		return
	}

	ret = &HttpEmailBridge{
		Hermes:        config.Hermes.ToHermes(),
		Body:          config.Hermes.Body.ToHermesBody(),
		Sender:        config.Sender.ToEmailSender(),
		Encryptor:     encryptor,
		root:          config.Root,
		pathStorage:   config.PathStorage,
		emailDataTmpl: emailDataFormTemplate,
	}

	if err = ret.checkAndCreateStorage(); err != nil {
		return
	}

	if err = ret.checkAndCreateStatic(); err != nil {
		return
	}

	if serveMux != nil {
		ret.WireRoutes(serveMux, &config.Routes)
	}

	return
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

func (o *HttpEmailBridge) GenerateEmailCode(w http.ResponseWriter, r *http.Request) {
	emailData := decodeEmailDataParams(r)

	if len(emailData.To) == 0 || emailData.Name == "" || emailData.Subject == "" {
		var emailDataForm bytes.Buffer
		if err := o.emailDataTmpl.Execute(&emailDataForm, emailData); err != nil {
			statusBadRequest(w, err.Error())
		} else {
			statusBadRequest(w, emailDataForm.String())
		}
	} else {
		logrus.Debugf("GenerateEmailCode, %v, %v", emailData.To, emailData.Subject)

		if data, err := o.EncryptInstance(emailData); err == nil {
			link := fmt.Sprintf("%v?%v=%v", emailData.Url, paramEmailCode, hex.EncodeToString(data))
			statusOk(w, fmt.Sprintf("<html><head><title>EmailCode</title></head><body><a target=\"_blank\" href=\"%v\">Link</a></br></br><textarea cols=\"50\" rows=\"10\">%v</textarea></body></html>", link, link))
		} else {
			statusBadRequest(w, err.Error())
		}
	}
	return
}

func (o *HttpEmailBridge) FaviconHandler(w http.ResponseWriter, r *http.Request) {
	favicon := fmt.Sprintf("%v/favicon.ico", o.root)
	http.ServeFile(w, r, favicon)
}

func (o *HttpEmailBridge) SendEmailByCode(w http.ResponseWriter, r *http.Request) {
	var emailData *EmailData
	if emailData = o.decryptEmailData(w, r); emailData == nil {
		return
	}

	o.sendEmailByEmailData(emailData, w, r)
}

func (o *HttpEmailBridge) sendEmailByEmailData(emailData *EmailData, w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("sendEmailByEmailData, %v, %v", emailData.To, emailData.Subject)

	message, err := o.BuildEmail(emailData.ToAsString(), emailData.Subject, o.BuildBody(net.GetQueryOrFormValue(paramMarkdown, r)))
	if err != nil {
		statusBadRequest(w, err.Error())
		return
	}

	o.storeEmail(r, message, emailData)

	if err := o.Send(message); err == nil {
		statusOk(w, "email sent successfully.")
	} else {
		statusBadRequest(w, err.Error())
	}
}

func decodeEmailDataParams(r *http.Request) (ret *EmailData) {
	ret = &EmailData{
		To:       strings.Split(net.GetQueryOrFormValue(paramTo, r), ","),
		Name:     net.GetQueryOrFormValue(paramName, r),
		Subject:  net.GetQueryOrFormValue(paramSubject, r),
		Url:      net.GetQueryOrFormValue(paramUrl, r),
		Markdown: net.GetQueryOrFormValue(paramMarkdown, r),

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

func (o *HttpEmailBridge) storeEmail(r *http.Request, htmlMessage *email.Message, emailData *EmailData) {
	if o.storeEmails {
		fileData := []byte(fmt.Sprintf("request:\n%v\n\nmessage:\n%v\n", net.FormatRequestFrom(r),
			htmlMessage.PlainText))
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
	if o.root != "" {
		if err = os.MkdirAll(o.root, 0755); err == nil {
			logrus.Infof("use the static path: %v", o.root)
		} else {
			err = errors.New("path for static files not defined")
		}
	}
	return
}

func (o *HttpEmailBridge) BuildBody(markdown string) (ret *hermes.Body) {

	ret = &*o.Body
	ret.FreeMarkdown = hermes.Markdown(markdown)
	return ret
}

func (o HttpEmailBridge) BuildEmail(to, subject string, body *hermes.Body) (ret *email.Message, err error) {

	hEmail := hermes.Email{
		Body: *body,
	}

	ret = &email.Message{To: to, Subject: subject}
	if ret.PlainText, err = o.GeneratePlainText(hEmail); err == nil {
		ret.HTML, err = o.GenerateHTML(hEmail)
	}
	return
}

func (o HttpEmailBridge) WireRoutes(serveMux *http.ServeMux, routes *Routes) {

	if routes.GenerateEmailCode != "" {
		route := routes.Prefix + routes.GenerateEmailCode
		logrus.Infof("add route, %v", route)
		serveMux.HandleFunc(route, o.GenerateEmailCode)
	}
	if routes.SendEmail != "" {
		route := routes.Prefix + routes.SendEmail
		logrus.Infof("add route, %v", route)
		serveMux.HandleFunc(routes.Prefix+routes.SendEmail, o.SendEmail)
	}
	if routes.SendEmailByCode != "" {
		route := routes.Prefix + routes.SendEmailByCode
		logrus.Infof("add route, %v", route)
		serveMux.HandleFunc(routes.Prefix+routes.SendEmailByCode, o.SendEmailByCode)
	}
	if routes.Favicon != "" {
		route := routes.Prefix + routes.Favicon
		logrus.Infof("add route, %v", route)
		serveMux.HandleFunc(routes.Favicon, o.FaviconHandler)
	}

	return
}
