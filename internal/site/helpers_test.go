package site

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// on sends one request to the handler, addressed to host, as it would arrive
// over TLS for that name.
func on(t *testing.T, method, host, target string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, "https://"+host+target, nil)
	r.Host = host
	r.TLS = &tls.ConnectionState{ServerName: hostName(host)}
	w := httptest.NewRecorder()
	Handler().ServeHTTP(w, r)
	return w
}

// get is a GET to denyfirst.dev.
func get(t *testing.T, target string) *httptest.ResponseRecorder {
	t.Helper()
	return on(t, http.MethodGet, siteName, target)
}

// pageBody is the body of a page this site renders.
func pageBody(t *testing.T, path string) string {
	t.Helper()
	w := get(t, path)
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s = %d", path, w.Code)
	}
	return w.Body.String()
}

func asset(t *testing.T, name string) string {
	t.Helper()
	body, err := assets.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func stylesheet(t *testing.T) string {
	t.Helper()
	return asset(t, "assets/style.css")
}

// cssRule is the body of the first rule whose selector is exactly selector.
func cssRule(t *testing.T, sheet, selector string) string {
	t.Helper()
	m := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(selector) + `\s*\{([^}]*)\}`).FindStringSubmatch(sheet)
	if m == nil {
		t.Fatalf("the stylesheet has no rule for %q", selector)
	}
	return m[1]
}

// between is what lies after open and before the next close.
func between(s, open, close string) (string, bool) {
	_, rest, ok := strings.Cut(s, open)
	if !ok {
		return "", false
	}
	inner, _, _ := strings.Cut(rest, close)
	return inner, true
}
