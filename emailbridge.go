package emailbridge

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/go-ee/utils/email"
	"github.com/go-ee/utils/encrypt"
	"github.com/go-ee/utils/net"
	"github.com/sirupsen/logrus"
	"html/template"
	"net/http"
	"os"
	"strings"
)

const (
	paramTo      = "to"
	paramName    = "name"
	paramSubject = "subject"
	paramUrl     = "url"

	paramEmailCode = "emailCode"
	paramMarkdown  = "Markdown"

	emailFormTemplateText = `
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

type HttpEmailBridge struct {
	*email.Engine
	*encrypt.Encryptor
	staticFolder  string
	emailDataTmpl *template.Template
}

func NewEmailBridge(config *Config, serveMux *http.ServeMux) (ret *HttpEmailBridge, err error) {

	config.Setup()

	var encryptor *encrypt.Encryptor

	if encryptor, err = encrypt.NewEncryptor(config.EncryptPassphrase); err != nil {
		return
	}

	var emailFormTemplate *template.Template

	if emailFormTemplate, err = template.New("test").
		Funcs(template.FuncMap{"StringsJoin": strings.Join}).Parse(emailFormTemplateText); err != nil {
		return
	}

	var emailEngine *email.Engine
	if emailEngine, err = email.NewEngine(&config.EngineConfig); err != nil {
		return
	}

	ret = &HttpEmailBridge{
		Engine:        emailEngine,
		Encryptor:     encryptor,
		staticFolder:  config.StaticFolder,
		emailDataTmpl: emailFormTemplate,
	}

	if err = ret.checkAndCreateStaticFolder(); err != nil {
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
	favicon := fmt.Sprintf("%v/favicon.ico", o.staticFolder)
	http.ServeFile(w, r, favicon)
}

func (o *HttpEmailBridge) SendEmailByCode(w http.ResponseWriter, r *http.Request) {
	var emailData *email.EmailData
	if emailData = o.decryptEmailData(w, r); emailData == nil {
		return
	}

	o.sendEmailByEmailData(emailData, w, r)
}

func (o *HttpEmailBridge) sendEmailByEmailData(emailData *email.EmailData, w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("sendEmailByEmailData, %v, %v", emailData.To, emailData.Subject)

	if err := o.Send(emailData); err == nil {
		statusOk(w, "email sent successfully.")
	} else {
		statusBadRequest(w, err.Error())
	}
}

func decodeEmailDataParams(r *http.Request) (ret *email.EmailData) {
	ret = &email.EmailData{
		To:       strings.Split(net.GetQueryOrFormValue(paramTo, r), ","),
		Name:     net.GetQueryOrFormValue(paramName, r),
		Subject:  net.GetQueryOrFormValue(paramSubject, r),
		Url:      net.GetQueryOrFormValue(paramUrl, r),
		Markdown: net.GetQueryOrFormValue(paramMarkdown, r)}
	return
}

func (o *HttpEmailBridge) decryptEmailData(w http.ResponseWriter, r *http.Request) (ret *email.EmailData) {
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

func (o *HttpEmailBridge) checkAndCreateStaticFolder() (err error) {
	if o.staticFolder != "" {
		if err = os.MkdirAll(o.staticFolder, 0755); err == nil {
			logrus.Infof("use the static path: %v", o.staticFolder)
		} else {
			err = errors.New("path for static files not defined")
		}
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
