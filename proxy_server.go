package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/google/uuid"
)

var (
	sendMsgPath = regexp.MustCompile("^/v1/Account/(MA|SA)[A-Z0-9]+/Message/$")
)

func spawnProxyServer(addr string) (*http.Server, error) {
	u, err := url.Parse(plivoURL)
	if err != nil {
		return nil, fmt.Errorf("url.Parse(%s) failed: %s", plivoURL, err.Error())
	}

	rproxy := httputil.NewSingleHostReverseProxy(u)
	defaultDirector := rproxy.Director
	rproxy.Director = wrapDirectors(interceptRequest, defaultDirector)
	rproxy.ModifyResponse = interceptResponse

	mux := http.NewServeMux()
	mux.Handle("/", rproxy)

	srv := &http.Server{
		Addr:           addr,
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   30 * time.Second,
		MaxHeaderBytes: 1 << 13,
	}

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("srv.ListenAndServe() failed: %v", err)
		}
	}()

	return srv, nil
}

func wrapDirectors(dirs ...func(*http.Request)) func(*http.Request) {
	return func(req *http.Request) {
		for _, d := range dirs {
			d(req)
		}
	}
}

func interceptRequest(req *http.Request) {
	if req.Method != http.MethodPost || !sendMsgPath.MatchString(req.URL.Path) {
		return
	}

	reqBytes, err := io.ReadAll(req.Body)
	if err != nil {
		log.Printf("io.ReadAll() failed: %v", err)
		return
	}

	defer func() {
		buf := bytes.NewBuffer(reqBytes)
		req.Body = io.NopCloser(buf)
		req.ContentLength = int64(len(reqBytes))
		req.Header.Set("Content-Length", strconv.Itoa(len(reqBytes)))
	}()

	var sendMsgReq BackendRequest
	if err := json.Unmarshal(reqBytes, &sendMsgReq); err != nil {
		log.Printf("json.Unmarshal() failed: %v", err)
		return
	}

	sendMsgReq.URL = callbackURL
	sendMsgReq.Method = http.MethodPost

	b, err := json.Marshal(sendMsgReq)
	if err != nil {
		log.Printf("json.Marshal() failed: %v", err)
		return
	}
	reqBytes = b
}

func interceptResponse(resp *http.Response) error {
	if resp.StatusCode != http.StatusAccepted || resp.Request.Method != http.MethodPost || !sendMsgPath.MatchString(resp.Request.URL.Path) {
		return nil
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("io.ReadAll() failed: %v", err)
		return fmt.Errorf("io.ReadAll() failed: %w", err)
	}

	defer func() {
		buf := bytes.NewBuffer(respBytes)
		resp.Body = io.NopCloser(buf)
		resp.Header.Set("Content-Length", strconv.Itoa(len(respBytes)))
		log.Printf("responding; body=%s", string(respBytes))
	}()

	var sendMsgResp BackendResponse
	if err := json.Unmarshal(respBytes, &sendMsgResp); err != nil {
		log.Printf("json.Unmarshal() failed: %v", err)
		return fmt.Errorf("json.Unmarshal() failed: %w", err)
	}

	if sendMsgResp.Message != "message(s) queued" || len(sendMsgResp.MessageUUID) != 1 {
		return nil
	}

	messageUUID := sendMsgResp.MessageUUID[0]

	if _, err := uuid.Parse(messageUUID); err != nil {
		log.Printf("uuid.Parse() failed: %v", err)
		return fmt.Errorf("uuid.Parse() failed: %w", err)
	}

	authID, authToken, ok := resp.Request.BasicAuth()
	if !ok {
		return nil
	}

	entry := Entry{
		CbkCh: make(chan FrontendResponse, 3),
	}

	store.Put(messageUUID, entry)
	defer store.Delete(messageUUID)

	pollCh := make(chan FrontendResponse, 1)
	var cancelPoll bool
	go func() {
		time.Sleep(500 * time.Millisecond)
		for i := 0; i < 5; i++ {
			if cancelPoll {
				break
			}
			syncResp, err := pollMessageUUID(messageUUID, authID, authToken)
			if err != nil {
				log.Printf("pollMessageUUID() failed: %s", err.Error())
				time.Sleep(1 * time.Second)
				continue
			}
			pollCh <- *syncResp
			break
		}
	}()

	select {
	case syncResp := <-entry.CbkCh:
		cancelPoll = true
		b, err := json.Marshal(syncResp)
		if err != nil {
			log.Printf("json.Marshal() failed: %v", err)
			return fmt.Errorf("json.Marshal() failed: %w", err)
		}
		respBytes = b
		return nil
	case syncResp := <-pollCh:
		b, err := json.Marshal(syncResp)
		if err != nil {
			log.Printf("json.Marshal() failed: %v", err)
			return fmt.Errorf("json.Marshal() failed: %w", err)
		}
		respBytes = b
		return nil
	case <-time.After(callbackWaitTimeout):
		return fmt.Errorf("timeout on waiting for callback")
	}
}

func pollMessageUUID(messageUUID, authID, authToken string) (*FrontendResponse, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/v1/Account/%s/Message/%s/", plivoURL, authID, messageUUID), nil)
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest() failed: %w", err)
	}
	req.SetBasicAuth(authID, authToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http.DefaultClient.Do() failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned non-2xx resp: %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("io.ReadAll() failed: %w", err)
	}

	var syncResp FrontendResponse
	if err := json.Unmarshal(b, &syncResp); err != nil {
		return nil, fmt.Errorf("json.Unmarshal() failed: %w", err)
	}

	if syncResp.MessageState == "queued" {
		return nil, fmt.Errorf("message still in queued state")
	}

	return &syncResp, nil
}
