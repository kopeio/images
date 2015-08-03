package l7proxy

type ProxyServer struct {
	listeners map[string]*Listener
}

func NewProxyServer() *ProxyServer {
	p := &ProxyServer{}
	p.listeners = make(map[string]*Listener)
	return p
}

func (p *ProxyServer) AddListener(listener *Listener) {
	endpoint := listener.endpoint
	p.listeners[endpoint] = listener
}

func (p *ProxyServer) ListenAndServe() error {
	errors := make(chan error, 10)
	for _, listener := range p.listeners {
		go func() {
			err := listener.listenAndServe()
			if err != nil {
				errors <- err
			}
		}()
	}

	err := <-errors
	return err
}
