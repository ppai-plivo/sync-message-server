package main

import (
	"context"
	"log"
	"os"
	"os/signal"
)

func main() {
	cbkSrv, err := spawnCbkServer(cbkSrvAddr)
	if err != nil {
		log.Fatalf("spawnCbkServer() failed: %v", err)
	}
	log.Printf("started reverse proxy; addr=%s", proxySrvAddr)

	proxySrv, err := spawnProxyServer(proxySrvAddr)
	if err != nil {
		log.Fatalf("spawnProxyServer() failed: %v", err)
	}
	log.Printf("started callback server; addr=%s", cbkSrvAddr)

	mainWait := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint

		if err := proxySrv.Shutdown(context.Background()); err != nil {
			log.Printf("proxySrv.Shutdown() failed: %s", err.Error())
		}
		log.Printf("stopped reverse proxy")

		if err := cbkSrv.Shutdown(context.Background()); err != nil {
			log.Printf("cbkSrv.Shutdown() failed: %s", err.Error())
		}
		log.Printf("stopped callback server")

		close(mainWait)
	}()

	<-mainWait
	log.Printf("fin")
}
