module github.com/go-ee/emailbridge

go 1.15

require (
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/go-ee/utils v0.0.0-20201104184309-5b62a7627986
	github.com/gorilla/schema v1.2.0 // indirect
	github.com/sirupsen/logrus v1.7.0
	github.com/urfave/cli/v2 v2.3.0
)

replace github.com/go-ee/utils => ../utils/
