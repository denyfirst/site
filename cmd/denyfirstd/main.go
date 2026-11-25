// Command denyfirstd serves denyfirst.dev and mta-sts.denyfirst.dev over TLS.
//
// It speaks TLS and nothing else. The names it serves are on the HSTS preload
// list, so no browser asks them anything over plain HTTP, and a program that
// could answer in the clear is one configuration slip from doing so.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/denyfirst/site/internal/site"
)

// version is the release this binary was built from, set by scripts/build.sh
// with -ldflags -X.
var version = "(unknown: not built by scripts/build.sh)"

const (
	// maxHeaderBytes caps request headers. Nothing here reads a body, and the
	// largest honest request is a browser's GET with its usual headers.
	maxHeaderBytes = 16 << 10

	// defaultMaxConnections is how many connections may be open at once:
	// generous for a site of a few small files, and far below what it takes
	// to exhaust the machine.
	defaultMaxConnections = 512

	// shutdownGrace is how long responses in flight have to finish once a
	// signal arrives. The largest file here is the typeface, in milliseconds.
	shutdownGrace = 10 * time.Second
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	flags := flag.NewFlagSet("denyfirstd", flag.ContinueOnError)
	var (
		listen = flags.String("listen", "127.0.0.1:8443",
			"address to listen on; loopback by default so an accidental start\n"+
				"\tis not immediately public")
		tlsCert        = flags.String("tls-cert", "", "path to a PEM certificate chain covering "+strings.Join(site.Names(), " and "))
		tlsKey         = flags.String("tls-key", "", "path to the matching private key")
		maxConnections = flags.Int("max-connections", defaultMaxConnections,
			"connections allowed to be open at once, before any request exists")
		showVersion = flags.Bool("version", false, "print the release, then exit")
	)
	if err := flags.Parse(args); err != nil {
		return 2
	}

	if *showVersion {
		fmt.Println("denyfirstd", version)
		return 0
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "unexpected arguments: %s\n", strings.Join(flags.Args(), " "))
		return 2
	}
	if *tlsCert == "" || *tlsKey == "" {
		fmt.Fprintln(os.Stderr, "-tls-cert and -tls-key are required: this server answers over TLS only")
		return 2
	}

	certs, err := newCertReloader(*tlsCert, *tlsKey, site.Names())
	if err != nil {
		fmt.Fprintf(os.Stderr, "loading the certificate: %v\n", err)
		return 1
	}

	srv := newServer(site.Handler(), certs)

	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		fmt.Fprintf(os.Stderr, "listening on %s: %v\n", *listen, err)
		return 1
	}
	listener = limitListener(listener, *maxConnections)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() {
		// The certificate is in TLSConfig.GetCertificate.
		errc <- srv.ServeTLS(listener, "", "")
	}()

	// The only line this process prints in normal operation. It names the
	// program, not a request.
	fmt.Fprintf(os.Stderr, "denyfirstd %s listening on https://%s\n", version, *listen)

	select {
	case err := <-errc:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "server stopped: %v\n", err)
			return 1
		}
		return 0
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "shutdown did not complete within %s: %v\n", shutdownGrace, err)
		_ = srv.Close()
		return 1
	}
	fmt.Fprintln(os.Stderr, "denyfirstd stopped")
	return 0
}

// newServer is the server with every limit set, apart from the listener.
func newServer(handler http.Handler, certs *certReloader) *http.Server {
	return &http.Server{
		Handler: handler,

		// ReadHeaderTimeout is the one that matters most. Without it, a
		// client can open a connection and send headers one byte at a time
		// for as long as it likes, holding a slot the whole while.
		ReadHeaderTimeout: 5 * time.Second,

		// ReadTimeout covers headers and body together, so a slow body gets
		// no more room than a slow header. Nothing here reads a body.
		ReadTimeout: 10 * time.Second,

		// WriteTimeout bounds a reader that takes the response a byte at a
		// time. The largest response is the typeface, under 100 kB.
		WriteTimeout: 20 * time.Second,

		// IdleTimeout closes kept-alive connections that have gone quiet.
		IdleTimeout: 60 * time.Second,

		MaxHeaderBytes: maxHeaderBytes,

		// The default logger writes lines such as "http: TLS handshake error
		// from 203.0.113.7" to standard error. That is a visitor's address in
		// a log, which this site undertakes not to keep, written without any
		// code here asking for it. The promise has to survive a library
		// default, so the logger is replaced rather than trusted to stay
		// quiet.
		ErrorLog: log.New(io.Discard, "", 0),

		// HTTP/2 is switched off: a non-nil, empty TLSNextProto is what stops
		// http.Server enabling it beside TLS. It would add a stream state
		// machine, flow control and HPACK — each of which has produced
		// denial-of-service classes — for a site of a few small files that
		// keep-alive over HTTP/1.1 covers entirely.
		TLSNextProto: map[string]func(*http.Server, *tls.Conn, http.Handler){},

		TLSConfig: tlsConfig(certs),
	}
}

// tlsConfig is the TLS this server offers.
func tlsConfig(certs *certReloader) *tls.Config {
	return &tls.Config{
		// Consulted per handshake, so a renewal that rewrites the files is
		// picked up without a restart, and a name this server does not serve
		// gets no certificate at all.
		GetCertificate: certs.get,

		MinVersion: tls.VersionTLS12,

		// Session resumption is off. A ticket is a token the client presents
		// on its next connection, and anyone watching the wire who sees the
		// same token twice learns that two connections are the same person,
		// across a change of address and across days. A full handshake each
		// time costs this site nothing worth having.
		SessionTicketsDisabled: true,

		// TLS 1.2 is offered with forward secrecy and authenticated
		// encryption only. TLS 1.3 has no other kind, and Go does not let its
		// suites be chosen.
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		},

		// HTTP/2 is not offered, so it is not advertised either.
		NextProtos: []string{"http/1.1"},
	}
}

// errUnservedName is what a handshake for any other name is refused with.
var errUnservedName = errors.New("no certificate for that name")

// certReloader serves the current certificate from disk, to the names it was
// given and no others.
//
// Certificates last weeks and are renewed by a timer, so the files are re-read
// when they change rather than once at startup.
//
// A handshake that names nothing, or names something else, is refused before
// any certificate is sent. A scanner sweeping addresses would otherwise be
// handed the certificate, and with it the names that live at this address;
// asking by name is how every browser connects, so nothing a person does is
// refused.
type certReloader struct {
	certPath, keyPath string
	names             map[string]bool

	mu       sync.RWMutex
	cert     *tls.Certificate
	certMod  time.Time
	keyMod   time.Time
	lastStat time.Time
}

// statInterval bounds how often the files are checked, so a busy server does
// not stat twice per handshake.
const statInterval = 30 * time.Second

func newCertReloader(certPath, keyPath string, names []string) (*certReloader, error) {
	r := &certReloader{certPath: certPath, keyPath: keyPath, names: map[string]bool{}}
	for _, n := range names {
		r.names[n] = true
	}
	if err := r.reload(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *certReloader) get(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if !r.names[strings.TrimSuffix(strings.ToLower(hello.ServerName), ".")] {
		return nil, errUnservedName
	}

	r.mu.RLock()
	cert, last := r.cert, r.lastStat
	r.mu.RUnlock()
	if time.Since(last) < statInterval {
		return cert, nil
	}

	if changed, err := r.changed(); err == nil && changed {
		// A failed reload leaves the previous certificate in place. Serving a
		// certificate that worked a moment ago beats refusing every
		// handshake because a renewal wrote a partial file.
		_ = r.reload()
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cert, nil
}

func (r *certReloader) changed() (bool, error) {
	certInfo, err := os.Stat(r.certPath)
	if err != nil {
		return false, err
	}
	keyInfo, err := os.Stat(r.keyPath)
	if err != nil {
		return false, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastStat = time.Now()
	return !certInfo.ModTime().Equal(r.certMod) || !keyInfo.ModTime().Equal(r.keyMod), nil
}

// reload reads the pair, and takes it only if it covers every name served: a
// renewal that dropped one would otherwise be served to every visitor of the
// name it dropped, who would be shown an error instead of the page.
func (r *certReloader) reload() error {
	certInfo, err := os.Stat(r.certPath)
	if err != nil {
		return err
	}
	keyInfo, err := os.Stat(r.keyPath)
	if err != nil {
		return err
	}
	cert, err := tls.LoadX509KeyPair(r.certPath, r.keyPath)
	if err != nil {
		return fmt.Errorf("reading %s and %s: %w", r.certPath, r.keyPath, err)
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return fmt.Errorf("reading %s: %w", r.certPath, err)
	}
	for name := range r.names {
		if err := leaf.VerifyHostname(name); err != nil {
			return fmt.Errorf("%s does not cover %s", r.certPath, name)
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.cert = &cert
	r.certMod = certInfo.ModTime()
	r.keyMod = keyInfo.ModTime()
	r.lastStat = time.Now()
	return nil
}

// limitListener caps how many connections are open at one time.
//
// A TLS handshake costs an elliptic curve operation before a byte of HTTP
// arrives, so a client that opens connections and does nothing never meets a
// request limit. ReadHeaderTimeout bounds how long each lasts; this bounds how
// many. A connection over the cap is closed at once rather than queued, so a
// flood does not become a slow version of itself with visitors waiting behind
// it.
func limitListener(inner net.Listener, limit int) net.Listener {
	if limit <= 0 {
		limit = defaultMaxConnections
	}
	return &limitedListener{Listener: inner, slots: make(chan struct{}, limit)}
}

type limitedListener struct {
	net.Listener
	slots chan struct{}
}

func (l *limitedListener) Accept() (net.Conn, error) {
	// A loop, so a flood is refused in constant space.
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		select {
		case l.slots <- struct{}{}:
			return &limitedConn{Conn: conn, release: l.release}, nil
		default:
			_ = conn.Close()
		}
	}
}

// release frees a slot without ever blocking: a slot leaked by a bug costs
// capacity, and a release that blocked would cost the listener.
func (l *limitedListener) release() {
	select {
	case <-l.slots:
	default:
	}
}

type limitedConn struct {
	net.Conn
	once    sync.Once
	release func()
}

func (c *limitedConn) Close() error {
	err := c.Conn.Close()
	// http.Server can close a connection more than once; the slot is
	// released once.
	c.once.Do(c.release)
	return err
}
