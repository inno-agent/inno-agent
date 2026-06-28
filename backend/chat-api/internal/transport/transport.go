package transport

import (
	"net"
	"net/http"
	"time"
)

func NewOptimized() *http.Transport {
	return &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		MaxConnsPerHost:     100,
		IdleConnTimeout:     120 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		WriteBufferSize:       64 * 1024,
		ReadBufferSize:        64 * 1024,
	}
}

func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: NewOptimized(),
		Timeout:   timeout,
	}
}
