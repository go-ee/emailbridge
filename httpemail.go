package emailbridge

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/go-ee/utils/encrypt"
	"github.com/go-ee/utils/net"
	"github.com/sirupsen/logrus"
	"net/http"
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
	port int, pathStatic string,
	encryptPassphrase string) (ret *HttpEmailBridge, err error) {

	var encryptor *encrypt.Encryptor

	if encryptor, err = encrypt.NewEncryptor(encryptPassphrase); err != nil {
		return
	}

	ret = &HttpEmailBridge{
		EmailSender: NewEmailSender(senderEmail, senderPassword),
		Encryptor:   encryptor,
		Port:        port,
		PathStatic:  pathStatic,
	}
	return
}

type HttpEmailBridge struct {
	*EmailSender
	*encrypt.Encryptor
	Port       int
	PathStatic string
}

func (o *HttpEmailBridge) Start() (err error) {
	http.HandleFunc("/favicon.ico", o.faviconHandler)
	http.HandleFunc("/generate", o.emailDataGenerate)
	http.HandleFunc("/", o.emailData)
	http.HandleFunc("/sendemail", o.sendEmail)
	serverAddr := fmt.Sprintf(":%v", o.Port)

	logrus.Infof("Start server at %v", serverAddr)
	err = http.ListenAndServe(serverAddr, nil)
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

	var emailBody string
	emailBody = net.GetQueryOrFormValue(paramEmailBody, r)

	htmlMessage := o.BuildHTMLEmail(emailData.To, emailData.Subject, emailBody)

	logrus.Infof("%v", htmlMessage)

	if err := o.SendMail(emailData.To, emailData.Subject, htmlMessage); err == nil {
		statusOk(w, "email sent successfully.")
	} else {
		statusBadRequest(w, err.Error())
	}
}

func statusOk(w http.ResponseWriter, msg string) {
	if _, resErr := w.Write([]byte(msg)); resErr != nil {
		logrus.Debug("error writing response %v", resErr)
	}
}

func statusBadRequest(w http.ResponseWriter, msg string) {
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
