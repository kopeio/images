package l7proxy

import "net/http"

func NewDummyBackendProvider() *DummyBackendProvider {
	d := &DummyBackendProvider{}
	return d
}

type DummyBackendProvider struct {
}

var _ BackendProvider = &DummyBackendProvider{}

func (d *DummyBackendProvider) PickBackend(r *http.Request, host string, backendCookie string, skip BackendIdList) *Backend {
	if !skip.Contains("2") {
		return &Backend{Id: "2", Endpoint: "invalid.justinsb.com"}
	}

	if !skip.Contains("1") {
		return &Backend{Id: "1", Endpoint: "blog.justinsb.com"}
	}
	return nil
}
