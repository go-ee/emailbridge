package emailbridge

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/go-ee/utils/encrypt"
	"github.com/go-ee/utils/net"
	"github.com/sirupsen/logrus"
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

	paramEmailCode = "emailCode"
	paramEmailBody = "emailBody"
)

func NewEmailData(to []string, name, subject string) *EmailData {
	return &EmailData{To: to, Name: name, Subject: subject, CreatedAt: time.Now()}
}

type EmailData struct {
	To        []string
	Name      string
	Subject   string
	CreatedAt time.Time
}

func NewEmailBridge(
	senderEmail, senderPassword string,
	port int, pathStorage string, pathStatic string,
	encryptPassphrase string) (ret *HttpEmailBridge, err error) {

	var encryptor *encrypt.Encryptor

	if encryptor, err = encrypt.NewEncryptor(encryptPassphrase); err != nil {
		return
	}

	ret = &HttpEmailBridge{
		EmailSender: NewEmailSender(senderEmail, senderPassword),
		Encryptor:   encryptor,
		Port:        port,
		PathStorage: pathStorage,
		PathStatic:  pathStatic,
	}
	return
}

type HttpEmailBridge struct {
	*EmailSender
	*encrypt.Encryptor
	Port        int
	PathStatic  string
	PathStorage string

	storeEmails bool
}

func (o *HttpEmailBridge) Start() (err error) {

	if err = o.checkAndCreateStorage(); err != nil {
		return
	}

	http.HandleFunc("/favicon.ico", o.faviconHandler)
	http.HandleFunc("/generate", o.emailDataGenerate)
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

func (o *HttpEmailBridge) emailDataGenerate(w http.ResponseWriter, r *http.Request) {
	if emailData := decodeEmailDataParams(w, r); emailData != nil {
		logrus.Debugf("emailDataGenerate, %v, %v", emailData.To, emailData.Subject)

		if data, err := o.EncryptInstance(emailData); err == nil {
			statusOk(w, fmt.Sprintf("<url>?%v=%v", paramEmailCode, hex.EncodeToString(data)))
		} else {
			statusBadRequest(w, err.Error())
		}
	}
}

func decodeEmailDataParams(w http.ResponseWriter, r *http.Request) (ret *EmailData) {
	var to, name, subject string
	if to = net.GetQueryOrFormValue(paramTo, r); to == "" {
		parameterNotProvided(w, paramTo)
		return
	}

	if name = net.GetQueryOrFormValue(paramName, r); name == "" {
		parameterNotProvided(w, paramName)
		return
	}

	if subject = net.GetQueryOrFormValue(paramSubject, r); subject == "" {
		parameterNotProvided(w, paramSubject)
		return
	}

	ret = NewEmailData(strings.Split(to, ","), name, subject)
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

	if o.storeEmails {
		fileData := []byte(fmt.Sprintf("request:\n%v\n\nmessage:\n%v\n", net.FormatRequestFrom(r), htmlMessage))
		filePath := filepath.Clean(fmt.Sprintf("%v/%v_%v.txt",
			o.PathStorage, strings.Join(emailData.To, "_"), emailData.Subject))
		if fileErr := ioutil.WriteFile(filePath, fileData, 0644); fileErr != nil {
			logrus.Warnf("can't write '%v', %v", filePath, fileErr)
		} else {
			logrus.Debugf("written '%v', bytes=%v", filePath, len(fileData))
		}
	}

	if err := o.SendMail(emailData.To, emailData.Subject, htmlMessage); err == nil {
		statusOk(w, "email sent successfully.")
	} else {
		statusBadRequest(w, err.Error())
	}
}

func statusOk(w http.ResponseWriter, msg string) {
	net.EnableCors(w)
	if _, resErr := w.Write([]byte(msg)); resErr != nil {
		logrus.Debug("error writing response %v", resErr)
	}
}

func statusBadRequest(w http.ResponseWriter, msg string) {
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

/*
func main() {
	email := &Email{To: "otschen.prosto@gmail.com", Subject: "Predigt von Thomas Höppel am 2.2.20"}
	if jsonData, err := json.Marshal(email); err == nil {
		ciphertext := encrypt(jsonData, "password")
		fmt.Printf("Encrypted: %x\n", ciphertext)
		plaintext := decrypt(ciphertext, "password")
		fmt.Printf("Decrypted: %s\n", plaintext)
		encryptFile("sample.txt", jsonData, "password1")
		fmt.Println(string(decryptFile("sample.txt", "password1")))
	}
}
*/
