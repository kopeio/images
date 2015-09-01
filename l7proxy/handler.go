package l7proxy

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"time"

	"github.com/golang/glog"
)

const BackendCookieName = "gravity"
const maxBackendAttempts = 4
const connectionTimeout = 3 * time.Second

type ProxyingHandler struct {
	backendProvider BackendProvider
	transport       *http.Transport
}

var _ http.Handler = &ProxyingHandler{}

func NewProxyingHandler(backendProvider BackendProvider) *ProxyingHandler {
	h := &ProxyingHandler{}
	h.backendProvider = backendProvider

	h.transport = &http.Transport{
		// This is the default http.Transport dialer, but with a replaced Timeout
		Dial: (&net.Dialer{
			Timeout:   connectionTimeout,
			KeepAlive: 30 * time.Second,
		}).Dial,
		// For now, we don't use connection pooling:
		DisableKeepAlives: true,
	}

	return h
}

// Returns the backend cookie, or "" if none is set
func findStickyBackendId(r *http.Request) string {
	cookie, err := r.Cookie(BackendCookieName)
	if err == nil {
		return cookie.Value
	}
	return ""
}

func (h *ProxyingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO: Support HTTP 2
	if r.ProtoMajor != 1 {
		http.Error(w, "Unsupported protocol", http.StatusBadRequest)
		return
	}

	proxiedRequest := &proxiedRequest{
		maxAttempts:     maxBackendAttempts,
		response:        w,
		request:         r,
		transport:       h.transport,
		backendProvider: h.backendProvider,
	}

	proxiedRequest.ServeHTTP()
}

type proxiedRequest struct {
	response        http.ResponseWriter
	request         *http.Request
	transport       *http.Transport
	backendProvider BackendProvider

	maxAttempts int
}

func (p *proxiedRequest) ServeHTTP() {
	reverseProxy := &httputil.ReverseProxy{
		Director: func(request *http.Request) {
			request.URL.Scheme = "http"
			request.URL.Host = p.request.Host
			// See https://github.com/cloudfoundry/gorouter/commit/96a7240d9c4247930e00155e08d0f1a11390a460
			request.URL.Opaque = p.request.RequestURI
			request.URL.RawQuery = ""
		},
		Transport:     p,
		FlushInterval: 20 * time.Millisecond,
		// TODO: ErrorLog?
	}

	reverseProxy.ServeHTTP(p.response, p.request)
}

// proxiedRequest implements http.RoundTripper, but adds retries
var _ http.RoundTripper = &proxiedRequest{}

func (p *proxiedRequest) RoundTrip(request *http.Request) (*http.Response, error) {
	var response *http.Response
	var err error

	var backend *Backend

	host := p.request.Host
	stickyBackendId := findStickyBackendId(p.request)

	// We need to buffer the body, in case we need to retry
	// TODO: Buffer to file
	// (this is also good for backends; we can absorb slow-uploads)
	var bufferedBody *os.File
	if request.Body != nil {
		// Switch between
		bufferedBody, err = ioutil.TempFile("", "body")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp file: %v", err)
		}

		defer func() {
			if bufferedBody != nil {
				err := bufferedBody.Close()
				if err != nil {
					// TODO: Only if backend didn't close?
					glog.Warning("error closing temp file: ", err)
				}

				err = os.Remove(bufferedBody.Name())
				if err != nil {
					glog.Warning("error removing temp file: ", err)
				}
			}
		}()
		_, err = io.Copy(bufferedBody, request.Body)
		if err != nil {
			glog.V(2).Info("error buffering body: ", err)
			return nil, fmt.Errorf("error buffering body: %v", err)
		}
	}

	attempt := 0
	var skip BackendIdList
	for {
		backend = p.backendProvider.PickBackend(p.request, host, stickyBackendId, skip)
		if backend == nil {
			// TODO: Behave differently if _no_ backends (i.e. host not configured?)
			glog.V(2).Info("could not connect to any backends for host: ", host)
			// TODO: I think this sends a StatusInternalError.  Should we send a more appropriate error?
			return nil, fmt.Errorf("no healthy backends")
		}

		request.URL.Host = backend.Endpoint
		if bufferedBody != nil {
			_, err := bufferedBody.Seek(0, 0)
			if err != nil {
				return nil, fmt.Errorf("error seeking to start of tempfile: %v", err)
			}
			request.Body = bufferedBody
		}

		response, err = p.transport.RoundTrip(request)
		if err == nil {
			break
		}

		if !p.canRetry(err) {
			break
		}

		attempt++
		if attempt >= p.maxAttempts {
			glog.V(2).Info("got retryable error, but retry limit already reached: ", err)
			break
		}

		glog.V(2).Info("will retry after retryable error: ", err)

		skip = append(skip, backend.Id)
	}

	if backend != nil && err == nil {
		cookie := &http.Cookie{
			Name:     BackendCookieName,
			Value:    backend.Id,
			HttpOnly: true,
		}

		http.SetCookie(p.response, cookie)
	}

	if err != nil {
		glog.V(2).Infof("error from backend for host %s: %v", host, err)
	}

	return response, err
}

// Checks if we should retry after the specified error from the backend
// We should only retry if the error was such that we can be sure no HTTP request happened
func (p *proxiedRequest) canRetry(e error) bool {
	switch e := e.(type) {
	case *net.OpError:
		if e.Op == "dial" {
			return true
		}
	}

	glog.V(2).Info("Will not retry after error: ", e)
	return false
}
