package site

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Every address this site answers itself, with what it answers.
func TestEveryAddressHereAnswers(t *testing.T) {
	want := map[string]string{
		"/":                     "text/html; charset=utf-8",
		"/privacy":              "text/html; charset=utf-8",
		"/robots.txt":           "text/plain; charset=utf-8",
		"/sitemap.xml":          "application/xml; charset=utf-8",
		"/llms.txt":             "text/plain; charset=utf-8",
		"/healthz":              "application/json; charset=utf-8",
		"/style.css":            "text/css; charset=utf-8",
		"/theme.js":             "text/javascript; charset=utf-8",
		"/hero.js":              "text/javascript; charset=utf-8",
		"/icon-denyfirst.svg":   "image/svg+xml",
		"/favicon.svg":          "image/svg+xml",
		"/favicon.ico":          "image/x-icon",
		"/denyfirst.ico":        "image/x-icon",
		"/apple-touch-icon.png": "image/png",
		"/denyfirst-touch.png":  "image/png",
		"/og-denyfirst.png":     "image/png",
		FontPath:                "font/woff2",
		SecurityTxtPath:         "text/plain; charset=utf-8",
		PGPKeyPath:              "text/plain; charset=utf-8",
	}
	for path, ct := range want {
		w := get(t, path)
		if w.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, w.Code)
			continue
		}
		if got := w.Header().Get("Content-Type"); got != ct {
			t.Errorf("GET %s is served as %q, want %q", path, got, ct)
		}
		if w.Body.Len() == 0 {
			t.Errorf("GET %s is empty", path)
		}
		if got := w.Header().Get("Content-Length"); got == "" {
			t.Errorf("GET %s says no length", path)
		}

		head := on(t, http.MethodHead, siteName, path)
		if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Length") != w.Header().Get("Content-Length") {
			t.Errorf("HEAD %s = %d with %d bytes and length %q, want 200, nothing, and GET's length",
				path, head.Code, head.Body.Len(), head.Header().Get("Content-Length"))
		}
	}

	// And nothing more: an address added to a table is added here too.
	var served []string
	for path := range rendered {
		served = append(served, path)
	}
	for path := range files {
		served = append(served, path)
	}
	for path := range plain {
		served = append(served, path)
	}
	served = append(served, healthPath)
	for _, path := range served {
		if _, ok := want[path]; !ok {
			t.Errorf("%s is served and not listed here", path)
		}
	}
}

// The health check says the program answers, and nothing about anybody.
func TestTheHealthCheckSaysOnlyThatItAnswers(t *testing.T) {
	w := get(t, healthPath)
	if w.Body.String() != "{\"status\":\"ok\"}\n" {
		t.Errorf("%s answers %q", healthPath, w.Body.String())
	}
	if on(t, http.MethodGet, mailPolicyName, healthPath).Code != http.StatusNotFound {
		t.Error("the policy name answers the health check; it answers the policy alone")
	}
}

// Every file in the embedded tree is served, rendered or read at startup, and
// every file the tables name is in it. A file nothing uses is a file nobody
// reviews.
func TestEveryEmbeddedFileIsUsedAndEveryUsedFileIsThere(t *testing.T) {
	used := map[string]bool{
		"assets/layout.html": true,
		"assets/llms.txt":    true,
		// The typeface's licence goes where the typeface goes.
		"assets/schibsted-grotesk-OFL.txt": true,
	}
	for _, p := range pages {
		used[p.Fragment] = true
	}
	for path, f := range files {
		if _, err := assets.ReadFile(f.name); err != nil {
			t.Errorf("%s is served from %s, which is not embedded", path, f.name)
		}
		used[f.name] = true
	}
	err := fs.WalkDir(assets, "assets", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !used[name] {
			t.Errorf("%s is embedded and nothing uses it", name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Every response carries the same headers, the redirects and refusals too.
func TestEveryResponseCarriesTheHeaders(t *testing.T) {
	cases := []struct{ method, host, path string }{
		{http.MethodGet, siteName, "/"},
		{http.MethodGet, siteName, "/privacy"},
		{http.MethodGet, siteName, "/style.css"},
		{http.MethodGet, siteName, "/healthz"},
		{http.MethodGet, siteName, "/llms.txt"},
		{http.MethodGet, siteName, "/security.txt"},
		{http.MethodGet, siteName, "/tls"},
		{http.MethodGet, siteName, "/organisation"},
		{http.MethodPost, siteName, "/"},
		{http.MethodPost, siteName, "/api/v1/scan"},
		{http.MethodGet, mailPolicyName, mailPolicyPath},
		{http.MethodGet, mailPolicyName, "/"},
		{http.MethodGet, "192.0.2.1", "/"},
	}
	want := map[string]string{
		"Content-Security-Policy":      contentSecurityPolicy,
		"X-Content-Type-Options":       "nosniff",
		"X-Frame-Options":              "DENY",
		"Referrer-Policy":              "no-referrer",
		"Cross-Origin-Resource-Policy": "same-origin",
		"Cross-Origin-Opener-Policy":   "same-origin",
		"Cross-Origin-Embedder-Policy": "require-corp",
		"Cache-Control":                "no-store",
		"Strict-Transport-Security":    "max-age=63072000; includeSubDomains; preload",
		"Permissions-Policy":           "accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()",
	}
	for _, c := range cases {
		w := on(t, c.method, c.host, c.path)
		for header, value := range want {
			if got := w.Header().Get(header); got != value {
				t.Errorf("%s %s%s: %s = %q, want %q", c.method, c.host, c.path, header, got, value)
			}
		}
		if w.Header().Get("Set-Cookie") != "" {
			t.Errorf("%s %s%s sets a cookie", c.method, c.host, c.path)
		}
	}

	// HSTS means something only over TLS, and is sent only there.
	r := httptest.NewRequest(http.MethodGet, "http://"+siteName+"/", nil)
	w := httptest.NewRecorder()
	Handler().ServeHTTP(w, r)
	if w.Header().Get("Strict-Transport-Security") != "" {
		t.Error("HSTS is sent over plain HTTP")
	}
}

// The policy lets a page load from this server and from nowhere else.
func TestContentSecurityPolicyAllowsOnlySelf(t *testing.T) {
	if !strings.HasPrefix(contentSecurityPolicy, "default-src 'none';") {
		t.Error("the policy does not deny by default")
	}
	for _, directive := range strings.Split(contentSecurityPolicy, ";") {
		fields := strings.Fields(directive)
		for _, source := range fields[1:] {
			switch source {
			case "'self'", "'none'", "'script'":
			default:
				t.Errorf("%s allows %s", fields[0], source)
			}
		}
	}
	for _, need := range []string{"frame-ancestors 'none'", "base-uri 'none'", "form-action 'none'", "require-trusted-types-for 'script'", "trusted-types 'none'"} {
		if !strings.Contains(contentSecurityPolicy, need) {
			t.Errorf("the policy lacks %s", need)
		}
	}
}

// No page carries an inline script, an inline style or an event handler: the
// policy above would refuse each, and a page that needed one would be a page
// the policy is loosened for.
func TestNoInlineStyleOrScript(t *testing.T) {
	for path := range pages {
		body := pageBody(t, path)
		for _, m := range regexp.MustCompile(`<script([^>]*)>`).FindAllStringSubmatch(body, -1) {
			if !strings.Contains(m[1], `src="/`) {
				t.Errorf("%s carries an inline script", path)
			}
		}
		if strings.Contains(body, "<style") {
			t.Errorf("%s carries a style element", path)
		}
		if regexp.MustCompile(`\sstyle=`).MatchString(body) {
			t.Errorf("%s carries a style attribute", path)
		}
		if m := regexp.MustCompile(`\son[a-z]+=`).FindString(body); m != "" {
			t.Errorf("%s carries an event handler: %s", path, m)
		}
		if strings.Contains(body, "javascript:") {
			t.Errorf("%s carries a javascript: address", path)
		}
	}
}

// The scripts build nothing from a string, which is what the policy's
// trusted-types directives promise a browser.
func TestTheScriptsFeedNoMarkupParser(t *testing.T) {
	for _, name := range []string{"assets/theme.js", "assets/hero.js"} {
		src := asset(t, name)
		for _, sink := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "eval(", "new Function", "setTimeout(\"", "createContextualFragment", "DOMParser"} {
			if strings.Contains(src, sink) {
				t.Errorf("%s uses %s", name, sink)
			}
		}
		if !strings.HasPrefix(strings.TrimSpace(strings.SplitN(src, "*/", 2)[1]), `"use strict";`) {
			t.Errorf("%s is not strict", name)
		}
	}
}

// Only GET and HEAD are answered here; anything else is refused, and says
// what is allowed.
func TestOnlyReadMethodsAreServed(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodOptions, "PROPFIND"} {
		for _, path := range []string{"/", "/privacy", "/style.css", SecurityTxtPath, "/security.txt", "/llms.txt", "/healthz"} {
			w := on(t, method, siteName, path)
			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s = %d, want 405", method, path, w.Code)
			}
			if w.Header().Get("Allow") != "GET, HEAD" {
				t.Errorf("%s %s says Allow %q", method, path, w.Header().Get("Allow"))
			}
		}
	}
}

// Every address of Porch's that this name ever answered goes to the same path
// at porch.denyfirst.dev, with its query, and keeps a POST a POST.
func TestPorchsAddressesGoToPorch(t *testing.T) {
	for _, path := range []string{
		"/tls", "/web", "/names", "/terms", "/docs", "/about", "/scanning", "/undertakings",
		"/api/v1/scan", "/app.js", "/explain.js", "/session.js", "/porch-icon.svg", "/icon-porch.svg",
		"/og-porch.png", "/porch.ico", "/nothing", "/privacy/", "/PRIVACY", "/tls/method",
	} {
		w := get(t, path)
		if w.Code != http.StatusMovedPermanently || w.Header().Get("Location") != PorchURL+path {
			t.Errorf("GET %s = %d to %q, want 301 to %s", path, w.Code, w.Header().Get("Location"), PorchURL+path)
		}
		w = on(t, http.MethodHead, siteName, path)
		if w.Code != http.StatusMovedPermanently {
			t.Errorf("HEAD %s = %d, want 301", path, w.Code)
		}
		w = on(t, http.MethodPost, siteName, path)
		if w.Code != http.StatusPermanentRedirect || w.Header().Get("Location") != PorchURL+path {
			t.Errorf("POST %s = %d to %q, want 308, which keeps the method", path, w.Code, w.Header().Get("Location"))
		}
	}

	for target, want := range map[string]string{
		"/tls?host=a.example": PorchURL + "/tls?host=a.example",
		"/porch":              PorchURL + "/",
		"/porch?x=1":          PorchURL + "/?x=1",
		"/organisation":       SiteURL + "/privacy",
		"/organisation?x=1":   SiteURL + "/privacy?x=1",
		"/security.txt":       SecurityTxtPath,
	} {
		w := get(t, target)
		if w.Code != http.StatusMovedPermanently || w.Header().Get("Location") != want {
			t.Errorf("GET %s = %d to %q, want 301 to %s", target, w.Code, w.Header().Get("Location"), want)
		}
	}
	if w := on(t, http.MethodPost, siteName, "/organisation"); w.Code != http.StatusPermanentRedirect {
		t.Errorf("POST /organisation = %d, want 308", w.Code)
	}
}

// A redirect's host is always one of the two constants, whatever the path:
// nothing a request carries can send a reader to another site.
func TestTheRedirectsStayOnTheirSites(t *testing.T) {
	for _, target := range []string{
		"//elsewhere.example", "//elsewhere.example/x", "/%2Felsewhere.example", "/%2F%2Felsewhere.example",
		"/\\elsewhere.example", "/.%2E/elsewhere", "/x?next=https://elsewhere.example", "/@elsewhere.example",
		"/%0d%0aSet-Cookie:%20a=b", "/https://elsewhere.example",
	} {
		r := httptest.NewRequest(http.MethodGet, "https://"+siteName+"/", nil)
		u, err := url.ParseRequestURI(target)
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		r.URL = u
		r.RequestURI = target
		r.Host = siteName
		w := httptest.NewRecorder()
		Handler().ServeHTTP(w, r)

		loc := w.Header().Get("Location")
		if loc == "" {
			continue
		}
		parsed, err := url.Parse(loc)
		if err != nil {
			t.Errorf("%s redirects to %q, which does not parse", target, loc)
			continue
		}
		if parsed.Host != "porch.denyfirst.dev" && parsed.Host != "denyfirst.dev" && parsed.Host != "" {
			t.Errorf("%s redirects off the sites, to %q", target, loc)
		}
		if strings.ContainsAny(loc, "\r\n") || w.Header().Get("Set-Cookie") != "" {
			t.Errorf("%s splits the response: %q", target, loc)
		}
	}
}

// A name this server does not serve, or an address, is told so, and given
// nothing of either site.
func TestAnyOtherNameIsMisdirected(t *testing.T) {
	for _, host := range []string{"192.0.2.1", "192.0.2.1:443", "porch.denyfirst.dev", "www.denyfirst.dev", "localhost", "denyfirst.dev.example", "", "[2001:db8::1]"} {
		for _, path := range []string{"/", "/privacy", mailPolicyPath, "/healthz"} {
			r := httptest.NewRequest(http.MethodGet, "https://"+siteName+path, nil)
			r.Host = host
			w := httptest.NewRecorder()
			Handler().ServeHTTP(w, r)
			if w.Code != http.StatusMisdirectedRequest {
				t.Errorf("%q%s = %d, want 421", host, path, w.Code)
			}
			if strings.Contains(w.Body.String(), "denyfirst") || strings.Contains(w.Body.String(), "STSv1") || w.Header().Get("Location") != "" {
				t.Errorf("%q%s says whose server this is: %q", host, path, w.Body.String())
			}
		}
	}

	// A Host that is not the name the handshake asked for.
	for _, c := range []struct{ sni, host string }{{mailPolicyName, siteName}, {siteName, mailPolicyName}} {
		r := httptest.NewRequest(http.MethodGet, "https://"+c.host+"/", nil)
		r.Host = c.host
		r.TLS.ServerName = c.sni
		w := httptest.NewRecorder()
		Handler().ServeHTTP(w, r)
		if w.Code != http.StatusMisdirectedRequest {
			t.Errorf("Host %s over a handshake for %s = %d, want 421", c.host, c.sni, w.Code)
		}
	}
}

// The Host header is read the way a browser writes it.
func TestTheNameIsReadAsABrowserWritesIt(t *testing.T) {
	for _, host := range []string{"denyfirst.dev", "DenyFirst.Dev", "denyfirst.dev.", "denyfirst.dev:443", "DENYFIRST.DEV.:443"} {
		w := on(t, http.MethodGet, host, "/privacy")
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "<h1>Privacy.</h1>") {
			t.Errorf("%s/privacy = %d, want the privacy page", host, w.Code)
		}
	}
}

// Every link on a page that leads to this site answers, and every jump within
// a page lands on something.
func TestEveryLinkHereAnswers(t *testing.T) {
	for path := range pages {
		body := pageBody(t, path)
		for _, m := range regexp.MustCompile(`(?:href|src|content)="([^"]+)"`).FindAllStringSubmatch(body, -1) {
			link := m[1]
			switch {
			case strings.HasPrefix(link, "#"):
				if !strings.Contains(body, `id="`+link[1:]+`"`) {
					t.Errorf("%s jumps to %s, which is not on it", path, link)
				}
				continue
			case strings.HasPrefix(link, SiteURL+"/"):
				link = strings.TrimPrefix(link, SiteURL)
			case strings.HasPrefix(link, "/") && !strings.HasPrefix(link, "//"):
			default:
				continue
			}
			target, fragment, _ := strings.Cut(link, "#")
			w := get(t, target)
			if w.Code != http.StatusOK && !(w.Code == http.StatusMovedPermanently && target == "/security.txt") {
				t.Errorf("%s links %s, which answers %d", path, link, w.Code)
				continue
			}
			if fragment != "" && !strings.Contains(w.Body.String(), `id="`+fragment+`"`) {
				t.Errorf("%s links %s, and %s has no %s", path, link, target, fragment)
			}
		}
	}
}

// The links that leave this site go to its two names, GitHub, or mail.
func TestEveryLinkElsewhereIsOneWeExpect(t *testing.T) {
	allowed := regexp.MustCompile(`^(https://porch\.denyfirst\.dev/|https://github\.com/denyfirst(/|$)|https://schema\.org/Organization$|mailto:hello@denyfirst\.dev$|https://denyfirst\.dev/)`)
	var seen []string
	for path := range pages {
		for _, m := range regexp.MustCompile(`(?:href|src|itemtype)="((?:https?:|mailto:)[^"]*)"`).FindAllStringSubmatch(pageBody(t, path), -1) {
			if !allowed.MatchString(m[1]) {
				t.Errorf("%s links %s", path, m[1])
			}
			if strings.HasPrefix(m[1], "http://") {
				t.Errorf("%s links %s in the clear", path, m[1])
			}
			seen = append(seen, m[1])
		}
	}
	sort.Strings(seen)
	if len(seen) < 10 {
		t.Errorf("only %d outside links were found, which is too few to be right: %v", len(seen), seen)
	}
}
