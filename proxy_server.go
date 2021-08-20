package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/google/uuid"
)

type myCtxKeyType string

var (
	sendMsgPath        = regexp.MustCompile("^/v1/Account/(MA|SA)[A-Z0-9]+/Message/$")
	reqIDCtxKey        = myCtxKeyType("proxy-req-id")
	errMsgStillInQueue = errors.New("message still in queue")
	pollClient         = &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   3 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:   true,
			MaxIdleConns:        100,
			IdleConnTimeout:     60 * time.Second,
			TLSHandshakeTimeout: 3 * time.Second,
		},
		Timeout: 6 * time.Second,
	}
)

func getReqID(ctx context.Context, key myCtxKeyType) string {
	if v := ctx.Value(key); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

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
	if req.Method != http.MethodPost || !sendMsgPath.MatchString(req.URL.Path) || req.Body == nil {
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

	reqID := uuid.NewString()
	sendMsgReq.URL = callbackURL + "/" + reqID
	sendMsgReq.Method = http.MethodPost

	b, err := json.Marshal(sendMsgReq)
	if err != nil {
		log.Printf("json.Marshal() failed: %v", err)
		return
	}
	reqBytes = b

	entry := Entry{
		CbkCh:     make(chan FrontendResponse, 3),
		CreatedAt: time.Now(),
	}
	store.Put(reqID, entry)
	ctx := context.WithValue(req.Context(), reqIDCtxKey, reqID)
	*req = *req.WithContext(ctx)
}

func interceptResponse(resp *http.Response) error {
	if resp.StatusCode != http.StatusAccepted || resp.Request.Method != http.MethodPost || !sendMsgPath.MatchString(resp.Request.URL.Path) {
		return nil
	}

	authID, authToken, ok := resp.Request.BasicAuth()
	if !ok {
		return nil
	}

	reqID := getReqID(resp.Request.Context(), reqIDCtxKey)
	if reqID == "" {
		return nil
	}

	entry, ok := store.Get(reqID)
	if !ok {
		return nil
	}
	defer store.Delete(reqID)
	cbkCh := entry.CbkCh

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

	pollCh := make(chan FrontendResponse, 1)

	pollCtx, cancelPoll := context.WithCancel(context.Background())
	defer cancelPoll()

	go func() {
		for i := 0; i < 10; i++ {
			syncResp, err := pollMessageUUID(pollCtx, messageUUID, authID, authToken)
			if err != nil {
				if errors.Is(err, errMsgStillInQueue) {
					time.Sleep(500 * time.Millisecond)
					continue
				}
				if errors.Is(err, context.Canceled) {
					return
				}
				log.Printf("pollMessageUUID() failed: %s", err.Error())
				return
			}
			pollCh <- *syncResp
			break
		}
	}()

	var syncResp *FrontendResponse
	select {
	case v := <-pollCh:
		syncResp = &v
	case v := <-cbkCh:
		syncResp = &v
		cancelPoll()
	case <-time.After(callbackWaitTimeout):
		return fmt.Errorf("timeout on waiting for callback")
	}

	b, err := json.Marshal(syncResp)
	if err != nil {
		log.Printf("json.Marshal() failed: %v", err)
		return fmt.Errorf("json.Marshal() failed: %w", err)
	}
	respBytes = b
	return nil
}

func pollMessageUUID(ctx context.Context, messageUUID, authID, authToken string) (*FrontendResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/v1/Account/%s/Message/%s/", plivoURL, authID, messageUUID), nil)
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest() failed: %w", err)
	}
	req.SetBasicAuth(authID, authToken)

	resp, err := pollClient.Do(req)
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
		return nil, errMsgStillInQueue
	}

	return &syncResp, nil
}
