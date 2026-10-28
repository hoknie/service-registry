package smtpfake

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"net"
	"net/textproto"
	"strings"
	"sync"
	"testing"
	"time"
)

type Letter struct {
	From string
	To   []string
	Data string
	Auth string
	TLS  bool
}

type Server struct {
	Addr     string
	listener net.Listener
	config   *tls.Config
	pool     *x509.CertPool
	mu       sync.Mutex
	letters  []Letter
	delay    time.Duration
	refuse   bool
}

func Start(t testing.TB) *Server {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cert, pool := selfSigned(t)
	s := &Server{Addr: l.Addr().String(), listener: l, config: &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}, pool: pool}
	go s.serve()
	t.Cleanup(func() { _ = l.Close() })
	return s
}

func (s *Server) RootCAs() *x509.CertPool { return s.pool }

func (s *Server) Letters() []Letter {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Letter(nil), s.letters...)
}

func (s *Server) SetDelay(d time.Duration) {
	s.mu.Lock()
	s.delay = d
	s.mu.Unlock()
}

func (s *Server) RefuseData(refuse bool) {
	s.mu.Lock()
	s.refuse = refuse
	s.mu.Unlock()
}

func (s *Server) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.session(conn)
	}
}

func (s *Server) session(conn net.Conn) {
	defer conn.Close()
	s.mu.Lock()
	delay := s.delay
	s.mu.Unlock()
	time.Sleep(delay)
	tc := textproto.NewConn(conn)
	_ = tc.PrintfLine("220 smtpfake ready")
	var letter Letter
	secure := false
	for {
		line, err := tc.ReadLine()
		if err != nil {
			return
		}
		verb, arg, _ := strings.Cut(line, " ")
		switch strings.ToUpper(verb) {
		case "EHLO", "HELO":
			if secure {
				_ = tc.PrintfLine("250-smtpfake\r\n250 AUTH PLAIN")
			} else {
				_ = tc.PrintfLine("250-smtpfake\r\n250-STARTTLS\r\n250 AUTH PLAIN")
			}
		case "STARTTLS":
			_ = tc.PrintfLine("220 go ahead")
			tlsConn := tls.Server(conn, s.config)
			if err := tlsConn.Handshake(); err != nil {
				return
			}
			conn, secure = tlsConn, true
			tc = textproto.NewConn(conn)
		case "AUTH":
			_, payload, _ := strings.Cut(arg, " ")
			raw, _ := base64.StdEncoding.DecodeString(payload)
			letter.Auth = strings.ReplaceAll(string(raw), "\x00", ":")
			_ = tc.PrintfLine("235 ok")
		case "MAIL":
			_, from, _ := strings.Cut(arg, ":")
			letter.From = strings.Trim(from, "<> ")
			_ = tc.PrintfLine("250 ok")
		case "RCPT":
			_, to, _ := strings.Cut(arg, ":")
			letter.To = append(letter.To, strings.Trim(to, "<> "))
			_ = tc.PrintfLine("250 ok")
		case "DATA":
			s.mu.Lock()
			refuse := s.refuse
			s.mu.Unlock()
			if refuse {
				_ = tc.PrintfLine("554 refused")
				continue
			}
			_ = tc.PrintfLine("354 go")
			data, err := tc.ReadDotBytes()
			if err != nil {
				return
			}
			letter.Data, letter.TLS = string(data), secure
			s.mu.Lock()
			s.letters = append(s.letters, letter)
			s.mu.Unlock()
			letter = Letter{}
			_ = tc.PrintfLine("250 queued")
		case "QUIT":
			_ = tc.PrintfLine("221 bye")
			return
		default:
			_ = tc.PrintfLine("250 ok")
		}
	}
}

func selfSigned(t testing.TB) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "smtpfake"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, DNSNames: []string{"localhost"},
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(parsed)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: parsed}, pool
}
