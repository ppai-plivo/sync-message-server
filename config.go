package main

import "time"

const (
	plivoURL            = "https://api.plivo.com"
	callbackURL         = "https://23d32c47c7e0.ngrok.io"
	cbkSrvAddr          = ":8091"
	proxySrvAddr        = ":8090"
	callbackWaitTimeout = 10 * time.Second
)
