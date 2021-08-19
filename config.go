package main

import "time"

const (
	plivoURL            = "https://api.plivo.com"
	callbackURL         = "https://cd9d378e5836.ngrok.io"
	cbkSrvAddr          = ":8091"
	proxySrvAddr        = ":8090"
	callbackWaitTimeout = 10 * time.Second
)
