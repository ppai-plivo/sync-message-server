package main

import "time"

const (
	proxySrvAddr = ":8090"
	cbkSrvAddr   = ":8091"
	plivoURL     = "https://api.plivo.com"

	callbackURL         = "https://e9c3-157-45-87-61.ngrok.io"
	callbackWaitTimeout = 10 * time.Second

	pollAttempts   = 8
	pollSeed       = 25 * time.Millisecond
	disablePolling = false
)
