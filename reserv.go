package reserv

import (
	"github.com/cevatbarisyilmaz/plistener"
	"math/rand"
	"net"
	"net/http"
	"time"
)

type Reserv struct {
	Server         *http.Server
	APIHandler     http.Handler
	HTTPListeners  []net.Listener
	HTTPSListeners []net.Listener
	Domain         string
	IndexLocation  string
	StaticDir      string
	APIDir         string
	KeyLocation    string
	CertLocation   string
}

func New(ips []net.IP) (res *Reserv, err error) {
	if ips == nil || len(ips) == 0 {
		ips = []net.IP{net.IPv4(127, 0, 0, 1)}
	}
	var httpListeners, httpsListeners []net.Listener
	for _, ip := range ips {
		for i := 80; true; i = 443 {
			var listener *net.TCPListener
			listener, err = net.ListenTCP("tcp", &net.TCPAddr{
				IP:   ip,
				Port: i,
			})
			if err != nil {
				return
			}
			pListener := plistener.New(listener)
			pListener.OnSpam = func(ip net.IP) {
				pListener.TempBan(ip, time.Now().Add(time.Hour*24*(1+time.Duration(rand.Intn(28)))))
			}
			if i == 443 {
				httpsListeners = append(httpsListeners, pListener)
				break
			}
			httpListeners = append(httpListeners, pListener)
		}
	}
	res = &Reserv{
		Server: &http.Server{
			ReadHeaderTimeout: time.Minute,
			IdleTimeout:       time.Minute,
			MaxHeaderBytes:    4096,
		},
		HTTPListeners:  httpListeners,
		HTTPSListeners: httpsListeners,
		IndexLocation:  "index.html",
		StaticDir:      "static",
		APIDir:         "api",
		KeyLocation:    "key.key",
		CertLocation:   "cert.cert",
	}
	return
}

func (res *Reserv) Run() error {
	if res.Server.Handler == nil {
		res.Server.Handler = newHandler(res)
	}
	ch := make(chan error, 1)
	for _, listener := range res.HTTPListeners {
		go func() {
			ch <- res.Server.Serve(listener)
		}()
	}
	if res.Domain != "" {
		for _, listener := range res.HTTPSListeners {
			go func() {
				ch <- res.Server.ServeTLS(listener, res.CertLocation, res.KeyLocation)
			}()
		}
	}
	err := <-ch
	_ = res.Server.Close()
	return err
}
