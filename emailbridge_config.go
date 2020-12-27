package emailbridge

import (
	"github.com/go-ee/utils/email"
	"os"

	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v2"
)

type Routes struct {
	Prefix            string `yaml:"prefix"`
	GenerateEmailCode string `yaml:"generateEmailCode"`
	SendEmail         string `yaml:"sendEmail"`
	SendEmailByCode   string `yaml:"sendEmailByCode"`
	Favicon           string `yaml:"favicon"`
}

type Config struct {
	Server            string             `yaml:"server", envconfig:"SERVER"`
	Port              int                `yaml:"port", envconfig:"PORT"`
	CORS              string             `yaml:"cors", envconfig:"CORS"`
	StaticFolder      string             `yaml:"staticFolder", envconfig:"STATIC_FOLDER"`
	EncryptPassphrase string             `yaml:"encryptPassphrase", envconfig:"ENCRYPT_PASSPHRASE"`
	EngineConfig      email.EngineConfig `yaml:"engineConfig"`
	Routes            Routes             `yaml:"routes"`
}

func ConfigLoad(configFile string, cfg *Config) (err error) {
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

	return
}

func (o *Config) Setup() {
	if o.EncryptPassphrase == "" {
		o.EncryptPassphrase = o.EngineConfig.Sender.Email + o.EngineConfig.Sender.SMTP.Password
	}
}

func (o *Config) WriteConfig(configFile string) (err error) {
	var file *os.File
	if file, err = os.Create(configFile); err != nil {
		return
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	if err = encoder.Encode(o); err != nil {
		return
	}
	err = encoder.Close()
	return
}

func BuildDefault() (ret *Config) {
	ret = &Config{
		Server: "",
		Port:   7070,
		Routes: Routes{
			Prefix:            "_api",
			GenerateEmailCode: "/email/code",
			SendEmailByCode:   "/email/code/send",
			SendEmail:         "/email/send",
			Favicon:           "/favicon.ico",
		},
	}
	return
}
