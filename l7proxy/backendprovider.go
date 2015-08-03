package l7proxy

import "net/http"

type BackendProvider interface {
	PickBackend(r *http.Request, host string, backendCookie string, skip BackendIdList) *Backend
}

type Backend struct {
	Id       string
	Endpoint string
}

type BackendIdList []string

func (b BackendIdList) Contains(s string) bool {
	if b == nil {
		return false
	}
	for i := range b {
		if b[i] == s {
			return true
		}
	}
	return false
}
