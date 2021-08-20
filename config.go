package main

import "time"

const (
	plivoURL            = "https://api.plivo.com"
	callbackURL         = "https://3499ae347de9.ngrok.io"
	cbkSrvAddr          = ":8091"
	proxySrvAddr        = ":8090"
	callbackWaitTimeout = 10 * time.Second
	pollInterval        = 500 * time.Millisecond
)
