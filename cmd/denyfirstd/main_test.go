package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/denyfirst/site/internal/site"
)

// issue writes a certificate for names, signed by a fresh authority, and
// returns the paths and the authority.
func issue(t *testing.T, dir string, names ...string) (certPath, keyPath string, roots *x509.CertPool) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ca := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test authority"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, _ := x509.ParseCertificate(caDER)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: names[0]},
		DNSNames:     names,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, leaf, caCert, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPath, keyPath = filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	roots = x509.NewCertPool()
	roots.AddCert(caCert)
	return certPath, keyPath, roots
}

// serve starts the server as main does, on a free loopback port.
func serve(t *testing.T, limit int) (addr string, roots *x509.CertPool, certs *certReloader) {
	t.Helper()
	certPath, keyPath, roots := issue(t, t.TempDir(), site.Names()...)
	certs, err := newCertReloader(certPath, keyPath, site.Names())
	if err != nil {
		t.Fatal(err)
	}
	srv := newServer(site.Handler(), certs)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.ServeTLS(limitListener(ln, limit), "", "") }()
	t.Cleanup(func() { _ = srv.Close() })
	return ln.Addr().String(), roots, certs
}

// A handshake is answered for the names served, and for nothing else: not an
// address, not another name, not a handshake that names nothing.
func TestTheHandshakeIsAnsweredOnlyForTheNamesServed(t *testing.T) {
	addr, roots, _ := serve(t, 0)
	for _, name := range site.Names() {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: name, RootCAs: roots})
		if err != nil {
			t.Errorf("a handshake for %s failed: %v", name, err)
			continue
		}
		_ = conn.Close()
	}
	for _, name := range []string{"", "porch.denyfirst.dev", "www.denyfirst.dev", "example.com"} {
		// #nosec G402 -- the test asks what the server sends to a client that
		// checks nothing; nothing is trusted on the strength of it.
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: name, InsecureSkipVerify: true})
		if err == nil {
			_ = conn.Close()
			t.Errorf("a handshake for %q was answered with a certificate", name)
		}
	}
}

// TLS 1.2 at least, HTTP/1.1 only, and no ticket that ties two visits
// together.
func TestTheTLSIsWhatItSays(t *testing.T) {
	addr, roots, _ := serve(t, 0)

	// #nosec G402 -- offered on purpose, to see it refused.
	if conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: "denyfirst.dev", RootCAs: roots, MinVersion: tls.VersionTLS10, MaxVersion: tls.VersionTLS11}); err == nil {
		_ = conn.Close()
		t.Error("TLS 1.1 is accepted")
	}

	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: "denyfirst.dev", RootCAs: roots, NextProtos: []string{"h2", "http/1.1"}})
	if err != nil {
		t.Fatal(err)
	}
	if p := conn.ConnectionState().NegotiatedProtocol; p != "http/1.1" {
		t.Errorf("the server chose %q; it speaks HTTP/1.1 alone", p)
	}
	_ = conn.Close()

	cache := tls.NewLRUClientSessionCache(4)
	for i := 0; i < 2; i++ {
		for _, version := range []uint16{tls.VersionTLS12, tls.VersionTLS13} {
			c, err := tls.Dial("tcp", addr, &tls.Config{ServerName: "denyfirst.dev", RootCAs: roots, ClientSessionCache: cache, MinVersion: version, MaxVersion: version})
			if err != nil {
				t.Fatal(err)
			}
			// TLS 1.3 sends its ticket after the handshake; reading once
			// lets it arrive, if one were sent.
			_ = c.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			_, _ = c.Read(make([]byte, 1))
			if c.ConnectionState().DidResume {
				t.Errorf("a TLS %x connection resumed an earlier one", version)
			}
			_ = c.Close()
		}
	}

}

// The site answers over the real server as the handler does in its tests.
func TestThePagesAnswerOverTheServer(t *testing.T) {
	addr, roots, _ := serve(t, 0)
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: roots},
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, addr)
		},
	}
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer transport.CloseIdleConnections()

	for target, want := range map[string]int{
		"https://denyfirst.dev/":                                http.StatusOK,
		"https://denyfirst.dev/privacy":                         http.StatusOK,
		"https://denyfirst.dev/tls":                             http.StatusMovedPermanently,
		"https://mta-sts.denyfirst.dev/.well-known/mta-sts.txt": http.StatusOK,
		"https://mta-sts.denyfirst.dev/":                        http.StatusNotFound,
	} {
		res, err := client.Get(target)
		if err != nil {
			t.Errorf("GET %s: %v", target, err)
			continue
		}
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
		if res.StatusCode != want {
			t.Errorf("GET %s = %d, want %d", target, res.StatusCode, want)
		}
		if res.Proto != "HTTP/1.1" {
			t.Errorf("GET %s was answered over %s", target, res.Proto)
		}
		if res.Header.Get("Strict-Transport-Security") == "" {
			t.Errorf("GET %s carries no HSTS over TLS", target)
		}
	}
}

// A certificate that does not cover every name is refused at startup, and a
// renewal that drops one is not taken: the one that worked stays.
func TestACertificateMustCoverEveryName(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath, _ := issue(t, dir, "denyfirst.dev")
	if _, err := newCertReloader(certPath, keyPath, site.Names()); err == nil || !strings.Contains(err.Error(), "mta-sts.denyfirst.dev") {
		t.Errorf("a certificate for denyfirst.dev alone was taken: %v", err)
	}

	certPath, keyPath, _ = issue(t, dir, site.Names()...)
	certs, err := newCertReloader(certPath, keyPath, site.Names())
	if err != nil {
		t.Fatal(err)
	}
	before := certs.cert

	// A renewal for one name: written, noticed, and refused.
	issue(t, dir, "denyfirst.dev")
	later := time.Now().Add(time.Minute)
	_ = os.Chtimes(certPath, later, later)
	_ = os.Chtimes(keyPath, later, later)
	certs.mu.Lock()
	certs.lastStat = time.Time{}
	certs.mu.Unlock()
	got, err := certs.get(&tls.ClientHelloInfo{ServerName: "mta-sts.denyfirst.dev"})
	if err != nil || got != before {
		t.Errorf("a renewal that dropped a name replaced the certificate that covered it (err %v)", err)
	}

	// A good renewal is taken without a restart.
	issue(t, dir, site.Names()...)
	later = later.Add(time.Minute)
	_ = os.Chtimes(certPath, later, later)
	_ = os.Chtimes(keyPath, later, later)
	certs.mu.Lock()
	certs.lastStat = time.Time{}
	certs.mu.Unlock()
	if got, err := certs.get(&tls.ClientHelloInfo{ServerName: "denyfirst.dev"}); err != nil || got == before {
		t.Errorf("a renewal covering every name was not taken (err %v)", err)
	}

	if _, err := certs.get(&tls.ClientHelloInfo{ServerName: "DenyFirst.Dev."}); err != nil {
		t.Errorf("a name in another case, with its trailing dot, was refused: %v", err)
	}
	if _, err := certs.get(&tls.ClientHelloInfo{}); !errors.Is(err, errUnservedName) {
		t.Errorf("a handshake naming nothing was given a certificate (err %v)", err)
	}
}

// Over the cap, a connection is closed at once, and a slot is free again
// when a connection ends.
func TestConnectionsOverTheCapAreClosed(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	limited := limitListener(ln, 1)
	defer limited.Close()
	accepted := make(chan net.Conn, 4)
	go func() {
		for {
			c, err := limited.Accept()
			if err != nil {
				return
			}
			accepted <- c
		}
	}()

	first, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	held := <-accepted

	second, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	_ = second.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := second.Read(make([]byte, 1)); !errors.Is(err, io.EOF) && !strings.Contains(err.Error(), "reset") {
		t.Errorf("a connection over the cap was not closed: %v", err)
	}

	_ = held.Close()
	_ = held.Close() // twice, as http.Server may; the slot is freed once
	_ = first.Close()
	third, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer third.Close()
	select {
	case c := <-accepted:
		_ = c.Close()
	case <-time.After(2 * time.Second):
		t.Error("a slot was not freed when its connection closed")
	}
}

// The server writes nothing about a visitor: its error log is discarded, and
// its timeouts are set.
func TestTheServerKeepsNoLogAndSetsItsLimits(t *testing.T) {
	certPath, keyPath, _ := issue(t, t.TempDir(), site.Names()...)
	certs, err := newCertReloader(certPath, keyPath, site.Names())
	if err != nil {
		t.Fatal(err)
	}
	srv := newServer(site.Handler(), certs)
	if srv.ErrorLog == nil || srv.ErrorLog.Writer() != io.Discard {
		t.Error("the server's error log is not discarded, and Go's default writes visitors' addresses")
	}
	if srv.ReadHeaderTimeout == 0 || srv.ReadTimeout == 0 || srv.WriteTimeout == 0 || srv.IdleTimeout == 0 || srv.MaxHeaderBytes == 0 {
		t.Error("a timeout or the header limit is unset")
	}
	if srv.TLSNextProto == nil || len(srv.TLSNextProto) != 0 {
		t.Error("HTTP/2 is not switched off")
	}
	if !srv.TLSConfig.SessionTicketsDisabled || srv.TLSConfig.MinVersion != tls.VersionTLS12 {
		t.Error("session tickets are on, or TLS below 1.2 is allowed")
	}
}

// Without a certificate it does not start: it answers over TLS or not at all.
func TestItDoesNotStartWithoutTLS(t *testing.T) {
	for _, args := range [][]string{{}, {"-tls-cert", "x.pem"}, {"-tls-key", "x.pem"}, {"-tls-cert", "a", "-tls-key", "b", "extra"}} {
		if code := run(args); code != 2 {
			t.Errorf("run(%q) = %d, want 2", args, code)
		}
	}
	if code := run([]string{"-tls-cert", "/nonexistent/c.pem", "-tls-key", "/nonexistent/k.pem"}); code != 1 {
		t.Errorf("run with missing files = %d, want 1", code)
	}
}
