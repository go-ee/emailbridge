package emailbridge

import (
	"github.com/go-ee/utils/email"
	"github.com/kelseyhightower/envconfig"
	"github.com/matcornic/hermes/v2"
	"gopkg.in/yaml.v2"
	"os"
)

type SMTP struct {
	Server   string `yaml:"server", envconfig:"SMTP_SERVER"`
	Port     int    `yaml:"port", envconfig:"SMTP_PORT"`
	User     string `yaml:"user", envconfig:"SMTP_USER"`
	Password string `yaml:"password", envconfig:"SMTP_PASSWORD"`
}

type Sender struct {
	Email    string `yaml:"port", envconfig:"SENDER_EMAIL"`
	Identity string `yaml:"identity", envconfig:"SENDER_IDENTITY"`
	SMTP     SMTP   `yaml:"smtp"`
}

func (o *Sender) ToEmailSender() *email.Sender {
	return &email.Sender{
		SenderEmail:    o.Email,
		SenderIdentity: o.Identity,
		SMTPServer:     o.SMTP.Server,
		SMTPPort:       o.SMTP.Port,
		SMTPUser:       o.SMTP.User,
		SMTPPassword:   o.SMTP.Password}
}

type Product struct {
	Name        string `yaml:"name", envconfig:"PRODUCT_NAME"`
	Link        string `yaml:"link", envconfig:"PRODUCT_LINK"`
	Logo        string `yaml:"logo", envconfig:"PRODUCT_LOGO"`
	Copyright   string `yaml:"copyright", envconfig:"PRODUCT_COPYRIGHT"`
	TroubleText string `yaml:"troubleText", envconfig:"PRODUCT_TROUBLE_TEXT"`
}

func (o *Product) ToHermesProduct() *hermes.Product {
	return &hermes.Product{
		Name:        o.Name,
		Link:        o.Link,
		Logo:        o.Logo,
		Copyright:   o.Copyright,
		TroubleText: o.TroubleText,
	}
}

type Hermes struct {
	Product            Product `yaml:"product"`
	DisableCSSInlining bool    `yaml:"disableCSSInlining"`
}

func (o *Hermes) ToHermes() *hermes.Hermes {
	return &hermes.Hermes{
		Product:            *o.Product.ToHermesProduct(),
		DisableCSSInlining: o.DisableCSSInlining,
	}
}

type Routes struct {
	Prefix            string `yaml:"prefix"`
	GenerateEmailCode string `yaml:"generateEmailCode"`
	SendEmail         string `yaml:"sendEmail"`
	SendEmailByCode   string `yaml:"sendEmailByCode"`
	Favicon           string `yaml:"favicon"`
}

type Config struct {
	Server            string `yaml:"server", envconfig:"SERVER"`
	Port              int    `yaml:"port", envconfig:"PORT"`
	CORS              string `yaml:"cors", envconfig:"CORS"`
	Root              string `yaml:"root", envconfig:"PATH_ROOT"`
	PathStorage       string `yaml:"pathStorage", envconfig:"PATH_STORAGE"`
	EncryptPassphrase string `yaml:"encryptPassphrase", envconfig:"ENCRYPT_PASSPHRASE"`
	Sender            Sender `yaml:"sender"`
	Hermes            Hermes `yaml:"hermes"`
	Routes            Routes `yaml:"routes"`
}

func LoadConfig(configFile string, cfg *Config) (err error) {
	var file *os.File
	if file, err = os.Open(configFile); err != nil {
		return
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	if err = decoder.Decode(cfg); err != nil {
		return
	}
	err = envconfig.Process("", cfg)

	if cfg.EncryptPassphrase == "" {
		cfg.EncryptPassphrase = cfg.Sender.Email + cfg.Sender.SMTP.Password
	}
	return
}

func WriteConfig(configFile string, cfg *Config) (err error) {
	var file *os.File
	if file, err = os.Create(configFile); err != nil {
		return
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	if err = encoder.Encode(cfg); err != nil {
		return
	}
	err = encoder.Close()
	return
}

func BuildDefault() (ret *Config) {
	ret = &Config{
		Server:      "0.0.0.0",
		Port:        8080,
		Root:        "templates",
		PathStorage: "storage",
		Sender: Sender{
			Email:    "info@example.com",
			Identity: "Info",
			SMTP: SMTP{
				Server:   "mail.example.com",
				Port:     465,
				User:     "info@example.com",
				Password: "changeMe",
			},
		},
		Hermes: Hermes{
			Product: Product{
				Name:      "ExampleProduct",
				Link:      "www.example.com",
				Logo:      "www.example.com/logo.svg",
				Copyright: "@ Example",
			},
		},
		Routes: Routes{
			Prefix:            "",
			GenerateEmailCode: "/email/code",
			SendEmailByCode:   "/email/code/send",
			SendEmail:         "/email/send",
			Favicon:           "/favicon.ico",
		},
	}
	return
}
