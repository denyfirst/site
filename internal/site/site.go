// Package site serves denyfirst.dev, the organisation's own pages, and
// mta-sts.denyfirst.dev, which serves the policy for mail to denyfirst.dev and
// nothing else.
//
// Until 2026-11 these were served by Porch's demonstration, one binary for
// three names. They are a program of their own now, on an address of their
// own, so that a fault in a product is not a fault in the organisation's
// pages, and the organisation can list a second product without living inside
// the first. Nothing moved for a reader: every address that is Porch's answers
// here with a permanent redirect to the same path at porch.denyfirst.dev.
//
// Everything is embedded in the binary. A handler that reads from disk has a
// path to get wrong, a directory that can be listed, and a traversal to
// defend; an embedded filesystem has fixed contents decided at build time, so
// none of those questions arise. The routes are an explicit table rather than
// a file server, for the same reason: listing what is allowed cannot be
// surprised by something nobody thought to forbid.
package site

import (
	"bytes"
	"embed"
	"encoding/xml"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/denyfirst/site/internal/promises"
)

//go:embed assets
var assets embed.FS

// The two addresses the pages link across: this site's, and Porch's.
const (
	SiteURL  = "https://denyfirst.dev"
	PorchURL = "https://porch.denyfirst.dev"
)

// The names this server answers to. The handshake refuses any other (see
// cmd/denyfirstd), and a request whose Host is another is told so.
const (
	siteName       = "denyfirst.dev"
	mailPolicyName = "mta-sts.denyfirst.dev"
)

// Names are the names a certificate for this server has to cover.
func Names() []string { return []string{siteName, mailPolicyName} }

// contentSecurityPolicy lets a page load what this server serves and nothing
// else.
//
// There is no 'unsafe-inline' anywhere, which is what makes this policy worth
// having, and why there is no inline style or script in any asset (a test
// checks). The two trusted-types directives say, where a browser can enforce
// it, that no script here feeds a markup parser: theme.js and hero.js build
// nothing from a string. Browsers that do not implement them ignore both.
const contentSecurityPolicy = "default-src 'none'; " +
	"script-src 'self'; " +
	"style-src 'self'; " +
	"img-src 'self'; " +
	"font-src 'self'; " +
	"connect-src 'self'; " +
	"form-action 'none'; " +
	"frame-ancestors 'none'; " +
	"base-uri 'none'; " +
	"require-trusted-types-for 'script'; " +
	"trusted-types 'none'"

// SecurityTxtPath is where RFC 9116 requires the file to be served.
const SecurityTxtPath = "/.well-known/security.txt"

// PGPKeyPath serves the key a reporter encrypts to.
const PGPKeyPath = "/pgp-key.txt"

// PGPFingerprint identifies the key at PGPKeyPath.
//
// A key served from this domain and identified only by this domain proves
// nothing: whoever takes the domain serves their own key beside their own
// fingerprint, and a reporter encrypts an unpublished vulnerability straight
// to them. The fingerprint is therefore also in SECURITY.md, which lives on
// GitHub behind a different account and a different set of credentials. A
// reporter compares the two; taking one is not taking both. A test fails if
// they disagree.
const PGPFingerprint = "75B7A18A89715E3775DBCA2EA8D994D1221AA045"

// FontPath is where the typeface is served.
const FontPath = "/schibsted-grotesk.woff2"

// page is one rendered document.
type page struct {
	Title       string
	Description string

	// Fragment names the file holding the body, and Data, when set, makes it
	// a template executed with that value before the layout wraps it.
	Fragment string
	Data     any

	// Canonical is the address the page states as its own, set at startup.
	Canonical string

	// SiteURL and PorchURL are what the layout puts in front of a link.
	SiteURL, PorchURL string

	// Coming is what the products menu names as coming soon.
	Coming []promises.Product

	// Body is filled in at startup. It is template.HTML because the fragment
	// is a file in this repository rather than anything a visitor supplied.
	Body template.HTML
}

// pages is the whole site.
var pages = map[string]*page{
	"/": {
		Title:       "denyfirst — independent security and privacy tools",
		Description: "denyfirst builds security and privacy tools that keep your data with you. Porch checks TLS, websites, mail and DNS.",
		Fragment:    "assets/home.html",
		Data:        homePage{Coming: promises.Coming},
	},
	// The organisation's promises, and this site's own facts. Each product's
	// page says what it adds, and this one links to each.
	"/privacy": {
		Title:       "Privacy — denyfirst",
		Description: "What denyfirst promises for everything it publishes, and how to check each one.",
		Fragment:    "assets/organisation.html",
		Data: organisationPage{
			Organisation: promises.Organisation,
			Products:     promises.Products,
		},
	},
}

// homePage is what assets/home.html reads.
type homePage struct {
	Coming []promises.Product
}

// organisationPage is what assets/organisation.html reads.
//
// The undertakings are passed in rather than written into the markup, so they
// are written once, in internal/promises, and the page and anything else that
// quotes them cannot disagree.
type organisationPage struct {
	Organisation []promises.Promise
	Products     []promises.Product
}

// servedFile is an asset served as it is.
type servedFile struct {
	name        string
	contentType string
}

// files are the assets served as they are.
var files = map[string]servedFile{
	"/style.css": {"assets/style.css", "text/css; charset=utf-8"},
	"/theme.js":  {"assets/theme.js", "text/javascript; charset=utf-8"},
	"/hero.js":   {"assets/hero.js", "text/javascript; charset=utf-8"},

	// The mark, at the address the pages link, and at the one they linked
	// before 2026-10-07, for a browser that kept it.
	"/icon-denyfirst.svg": {"assets/denyfirst.svg", "image/svg+xml"},
	"/favicon.svg":        {"assets/denyfirst.svg", "image/svg+xml"},

	// For what reads no SVG: an .ico of the mark at 16, 32 and 48 pixels, and
	// a square of it at 180 for a phone's home screen. Browsers ask every name
	// for /favicon.ico and /apple-touch-icon.png whatever a page links.
	"/favicon.ico":          {"assets/denyfirst.ico", "image/x-icon"},
	"/denyfirst.ico":        {"assets/denyfirst.ico", "image/x-icon"},
	"/apple-touch-icon.png": {"assets/denyfirst-touch.png", "image/png"},
	"/denyfirst-touch.png":  {"assets/denyfirst-touch.png", "image/png"},

	// The one typeface every page is set in, served from here so a reader's
	// browser asks nobody else for it. Schibsted Grotesk, under the SIL Open
	// Font License 1.1 (schibsted-grotesk-OFL.txt beside it).
	FontPath: {"assets/schibsted-grotesk.woff2", "font/woff2"},

	// The picture a link preview shows, at the 1200 by 630 the layout states.
	// It carries no metadata, which a test checks.
	"/og-denyfirst.png": {"assets/og-denyfirst.png", "image/png"},

	// Where to report a security problem, and the key to encrypt it to.
	SecurityTxtPath: {"assets/security.txt", "text/plain; charset=utf-8"},
	// text/plain rather than application/pgp-keys, so a browser shows it
	// instead of offering to download a file a reporter then has to find.
	PGPKeyPath: {"assets/pgp-key.txt", "text/plain; charset=utf-8"},
}

// moved are addresses here that answer with another address here.
//
// RFC 9116 puts security.txt under /.well-known/ and treats the top-level path
// as legacy, but a person looking for a way to report a vulnerability will try
// the short one, and a 404 costs a report. A redirect rather than two copies
// keeps the canonical address in the file true.
var moved = map[string]string{
	"/security.txt": SecurityTxtPath,
}

// renamed are addresses this site used to have under another name. Answered
// like a redirect to Porch, keeping the query and, for anything but GET and
// HEAD, the method.
var renamed = map[string]string{
	// The organisation's privacy page was at /organisation while one program
	// served both names.
	"/organisation": "/privacy",
}

// healthPath answers a monitor. It is about the program, not a page.
const healthPath = "/healthz"

// rendered holds every page as finished bytes, and plain the crawler files.
//
// Built once at startup rather than per request: nothing here varies per
// visitor, so nothing here is assembled per visitor.
var (
	rendered = map[string][]byte{}
	plain    = map[string]plainFile{}
)

// plainFile is a file a crawler reads.
type plainFile struct {
	contentType string
	body        []byte
}

func init() {
	layout, err := template.ParseFS(assets, "assets/layout.html")
	if err != nil {
		panic("site: " + err.Error())
	}
	for path, p := range pages {
		body, err := render(layout, path, p)
		if err != nil {
			// At startup, so a broken page stops the process instead of
			// reaching a visitor with a hole in it.
			panic("site: rendering " + path + ": " + err.Error())
		}
		rendered[path] = body
	}
	buildPlain()
}

// canonicalURL is the one address a page states as its own.
func canonicalURL(path string) string {
	return SiteURL + path
}

func render(layout *template.Template, path string, p *page) ([]byte, error) {
	p.Canonical = canonicalURL(path)
	p.SiteURL, p.PorchURL = SiteURL, PorchURL
	p.Coming = promises.Coming

	fragment, err := assets.ReadFile(p.Fragment)
	if err != nil {
		return nil, err
	}
	if p.Data != nil {
		// Parsed as a template, and its values escaped by html/template on the
		// way in. They come from this repository either way; the escaping is
		// the reason a sentence containing an angle bracket cannot silently
		// become markup.
		body, err := template.New(p.Fragment).Option("missingkey=error").Parse(string(fragment))
		if err != nil {
			return nil, err
		}
		var filled bytes.Buffer
		if err := body.Execute(&filled, p.Data); err != nil {
			return nil, err
		}
		fragment = filled.Bytes()
	}
	p.Body = template.HTML(fragment) // #nosec G203 -- a file in this repository, not anything a visitor sent

	var out bytes.Buffer
	if err := layout.Execute(&out, p); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// buildPlain writes robots.txt, the sitemap and llms.txt.
//
// The site wants to be found, and the pages it wants found are the ones in the
// table, so the sitemap is that table rather than a list somebody keeps in
// step by hand.
func buildPlain() {
	var urls []string
	for path := range rendered {
		urls = append(urls, canonicalURL(path))
	}
	sort.Strings(urls)

	var sitemap strings.Builder
	sitemap.WriteString(xml.Header)
	sitemap.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, u := range urls {
		sitemap.WriteString("  <url><loc>" + u + "</loc></url>\n")
	}
	sitemap.WriteString("</urlset>\n")
	plain["/sitemap.xml"] = plainFile{"application/xml; charset=utf-8", []byte(sitemap.String())}
	plain["/robots.txt"] = plainFile{"text/plain; charset=utf-8",
		[]byte("User-agent: *\nAllow: /\n\nSitemap: " + SiteURL + "/sitemap.xml\n")}
	plain["/llms.txt"] = plainFile{"text/plain; charset=utf-8", buildLLMs()}
}

// Handler serves every name this server answers to.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := hostName(r.Host)

		// A request whose Host names something other than the name its
		// handshake asked for is answered by neither. Over HTTP/1.1, which is
		// all this server speaks, no browser sends one; something that does is
		// probing which names live together.
		if r.TLS != nil && r.TLS.ServerName != "" && hostName(r.TLS.ServerName) != name {
			misdirected(w, r)
			return
		}

		switch name {
		case siteName:
			serveSite(w, r)
		case mailPolicyName:
			// Answered here and never passed on: the pages are not served at
			// this name, so a sender fetching the policy gets the policy and
			// anybody else gets nothing.
			serveMailPolicy(w, r)
		default:
			misdirected(w, r)
		}
	})
}

// hostName reads a Host header the way a browser writes it: any case, an
// optional port, an optional trailing dot.
func hostName(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.TrimSuffix(strings.ToLower(host), ".")
}

// misdirected answers a request addressed to a name this server does not
// serve, or to an address: 421, which says so, rather than a page that would
// tell a scanner sweeping addresses whose server this is.
func misdirected(w http.ResponseWriter, r *http.Request) {
	setHeaders(w, r)
	http.Error(w, "This server does not answer to that name.", http.StatusMisdirectedRequest)
}

// own is every path denyfirst.dev answers itself. Anything else is Porch's,
// and redirected there.
func own(path string) bool {
	if _, ok := rendered[path]; ok {
		return true
	}
	if _, ok := files[path]; ok {
		return true
	}
	if _, ok := plain[path]; ok {
		return true
	}
	if _, ok := moved[path]; ok {
		return true
	}
	return path == healthPath
}

func serveSite(w http.ResponseWriter, r *http.Request) {
	setHeaders(w, r)
	path := r.URL.Path

	if to, ok := renamed[path]; ok {
		elsewhere(w, r, SiteURL, to)
		return
	}
	if !own(path) {
		// Porch's front page was at /porch on this name, and is at "/" on its
		// own.
		if path == "/porch" {
			path = "/"
		}
		elsewhere(w, r, PorchURL, path)
		return
	}

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Only GET and HEAD are served here.", http.StatusMethodNotAllowed)
		return
	}

	if to, ok := moved[path]; ok {
		http.Redirect(w, r, to, http.StatusMovedPermanently)
		return
	}
	if path == healthPath {
		write(w, r, "application/json; charset=utf-8", []byte(`{"status":"ok"}`+"\n"))
		return
	}
	if body, ok := rendered[path]; ok {
		write(w, r, "text/html; charset=utf-8", body)
		return
	}
	if f, ok := plain[path]; ok {
		write(w, r, f.contentType, f.body)
		return
	}
	file := files[path]
	body, err := assets.ReadFile(file.name)
	if err != nil {
		// Unreachable unless the table and the embedded tree disagree, which
		// a test checks.
		http.Error(w, "That page is unavailable.", http.StatusInternalServerError)
		return
	}
	write(w, r, file.contentType, body)
}

// elsewhere redirects to path, and the request's query, on the name base is.
//
// The target is one of two constant addresses with a path after it, so it
// cannot be steered off them: a path of "//elsewhere.example" becomes a path
// on porch.denyfirst.dev. GET and HEAD are answered with 301, which every
// client and search engine understands; anything else with 308, which keeps
// the method and the body, so a POST to the old address is not silently
// turned into a GET.
func elsewhere(w http.ResponseWriter, r *http.Request, base, path string) {
	target := url.URL{
		Scheme:   "https",
		Host:     strings.TrimPrefix(base, "https://"),
		Path:     path,
		RawQuery: r.URL.RawQuery,
	}
	if path == r.URL.Path {
		// As it arrived, so an escaped character stays escaped.
		target.RawPath = r.URL.RawPath
	}
	status := http.StatusPermanentRedirect
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		status = http.StatusMovedPermanently
	}
	// #nosec G710 -- the scheme and host are one of the two constants above,
	// never the request's; only the path and query are, and with a host in
	// front a path cannot name another site (TestTheRedirectsStayOnTheirSites).
	http.Redirect(w, r, target.String(), status)
}

func write(w http.ResponseWriter, r *http.Request, contentType string, body []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(body)
}

func setHeaders(w http.ResponseWriter, r *http.Request) {
	h := w.Header()

	h.Set("Content-Security-Policy", contentSecurityPolicy)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
	h.Set("Cross-Origin-Opener-Policy", "same-origin")

	// The privacy page promises nothing loaded from elsewhere. That holds
	// because nobody has added anything; this makes a browser refuse the
	// resource if somebody does: same-origin subresources need nothing
	// extra, so it costs nothing today and fails loudly the first time it
	// would stop being true.
	h.Set("Cross-Origin-Embedder-Policy", "require-corp")

	h.Set("Permissions-Policy",
		"accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()")

	// The whole site is a few kilobytes, so fetching it again costs almost
	// nothing, and a cache that holds nothing cannot leak anything.
	h.Set("Cache-Control", "no-store")

	if r.TLS != nil {
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
	}
}
