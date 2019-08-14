package reserv

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ipRecord struct {
	mu           *sync.Mutex
	blockedUntil *time.Time
	firstRequest time.Time
	requestCount int
}

type defaultHandler struct {
	Host string
	TLS  bool

	muxer *http.ServeMux

	recordsMu *sync.RWMutex
	records   map[[16]byte]*ipRecord
}

func newHandler(res *Reserv) *defaultHandler {
	muxer := http.NewServeMux()
	muxer.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		http.ServeFile(writer, request, res.IndexLocation)
	})
	muxer.Handle("/" + res.StaticDir + "/", http.StripPrefix("/" + res.StaticDir + "/", http.FileServer(http.Dir(res.StaticDir + "/"))))
	muxer.Handle("/" + res.APIDir + "/", http.StripPrefix("/" + res.APIDir, res.APIHandler))
	return &defaultHandler{
		Host:      res.Domain,
		TLS:       res.Domain != "" && len(res.HTTPSListeners) > 0,
		muxer:     muxer,
		recordsMu: &sync.RWMutex{},
		records:   map[[16]byte]*ipRecord{},
	}
}

func (handler *defaultHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if handler.checkLimit(writer, request) {
		return
	}
	if handler.Host != "" && handler.checkHost(writer, request) {
		return
	}
	if handler.TLS && handler.checkHTTPS(writer, request) {
		return
	}
	if handler.Host != "" && handler.checkWWW(writer, request) {
		return
	}
	handler.muxer.ServeHTTP(writer, request)
}

func (handler *defaultHandler) checkLimit(writer http.ResponseWriter, request *http.Request) (prevent bool) {
	defer func() {
		if prevent {
			writer.WriteHeader(http.StatusTooManyRequests)
		}
	}()
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return true
	}
	ip = ip.To16()
	handler.recordsMu.RLock()
	var ipByte [16]byte
	copy(ipByte[:], ip)
	record := handler.records[ipByte]
	if record == nil {
		handler.recordsMu.RUnlock()
		handler.recordsMu.Lock()
		record = handler.records[ipByte]
		if record == nil {
			record = &ipRecord{mu: &sync.Mutex{}, blockedUntil: nil, firstRequest: time.Now(), requestCount: 1}
			handler.records[ipByte] = record
			handler.recordsMu.Unlock()
			return false
		} else {
			handler.recordsMu.Unlock()
		}
	} else {
		handler.recordsMu.RUnlock()
	}
	now := time.Now()
	record.mu.Lock()
	defer record.mu.Unlock()
	if record.blockedUntil != nil {
		if now.After(*record.blockedUntil) {
			record.blockedUntil = nil
		} else {
			return true
		}
	}
	if record.firstRequest.Add(time.Hour * 24 * 7).Before(now) {
		record.firstRequest = now
		record.requestCount = 1
		return false
	}
	record.requestCount++
	if float64(record.requestCount)/(1000+now.Sub(record.firstRequest).Seconds()) > 0.1 {
		d := time.Minute + now.Sub(record.firstRequest)
		t := now.Add(d)
		writer.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(d.Seconds()))))
		record.blockedUntil = &t
		return true
	}
	return false
}

func (handler *defaultHandler) checkHost(writer http.ResponseWriter, request *http.Request) bool {
	if request.Host != handler.Host {
		writer.WriteHeader(http.StatusBadRequest)
		return true
	}
	return false
}

func (handler *defaultHandler) checkHTTPS(writer http.ResponseWriter, request *http.Request) bool {
	if request.TLS == nil {
		request.URL.Scheme = "https"
		request.URL.Host = request.Host
		http.Redirect(writer, request, request.URL.String(), http.StatusMovedPermanently)
		return true
	}
	return false
}

func (handler *defaultHandler) checkWWW(writer http.ResponseWriter, request *http.Request) bool {
	domains := strings.Split(request.Host, ".")
	if len(domains) == 2 {
		request.URL.Host = "www." + request.Host
		http.Redirect(writer, request, request.URL.String(), http.StatusMovedPermanently)
		return true
	}
	return false
}
