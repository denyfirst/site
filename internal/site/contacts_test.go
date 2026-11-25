package site

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// expiryMargin is how long before the Expires date these tests start failing.
//
// The point is that the date is moved by somebody who has re-read the file,
// not by whoever happens to be on call the week it lapses. Sixty days is
// enough to notice a failing build, decide the contacts are still right, and
// merge a change without hurrying.
const expiryMargin = 60 * 24 * time.Hour

// maxLifetime is the longest an Expires date may be set into the future.
//
// RFC 9116 says the value should be less than a year away. A file that
// promises a working contact address for three years is making a promise
// about a mailbox nobody has checked.
const maxLifetime = 365 * 24 * time.Hour

// denyfirstFile is one of our contact files as a reporter receives it.
func denyfirstFile(t *testing.T, path string) string {
	t.Helper()
	w := get(t, path)
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s returned %d, want 200", path, w.Code)
	}
	if got := w.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("%s is served as %q, want text/plain; charset=utf-8", path, got)
	}
	return w.Body.String()
}

// securityTxtFields parses the file the way a reporter's tool would: comments
// and blank lines dropped, field names lowercased, values kept in order of
// appearance.
func securityTxtFields(t *testing.T) map[string][]string {
	t.Helper()
	body := denyfirstFile(t, SecurityTxtPath)
	if !strings.HasSuffix(body, "\n") {
		t.Error("security.txt does not end with a line break; RFC 9116 ends every line with one")
	}
	fields := map[string][]string{}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, found := strings.Cut(line, ":")
		if !found {
			t.Errorf("line is neither a comment nor a field: %q", line)
			continue
		}
		name = strings.ToLower(strings.TrimSpace(name))
		value = strings.TrimSpace(value)
		if value == "" {
			t.Errorf("field %q has no value", name)
			continue
		}
		fields[name] = append(fields[name], value)
	}
	return fields
}

// A reporter who guesses the old location is sent to the file, not shown a
// 404.
func TestTheShortSecurityTxtPathLeadsToTheFile(t *testing.T) {
	w := get(t, "/security.txt")
	if w.Code != http.StatusMovedPermanently || w.Header().Get("Location") != SecurityTxtPath {
		t.Errorf("GET /security.txt = %d to %q, want 301 to %s", w.Code, w.Header().Get("Location"), SecurityTxtPath)
	}
}

func TestSecurityTxtHasTheRequiredFields(t *testing.T) {
	fields := securityTxtFields(t)
	if len(fields["contact"]) == 0 {
		t.Error("no Contact field; RFC 9116 requires at least one")
	}
	if n := len(fields["expires"]); n != 1 {
		t.Errorf("%d Expires fields, want exactly 1", n)
	}
	for _, contact := range fields["contact"] {
		if !strings.HasPrefix(contact, "mailto:") && !strings.HasPrefix(contact, "https://") && !strings.HasPrefix(contact, "tel:") {
			t.Errorf("Contact %q is not a URI; RFC 9116 requires mailto:, https: or tel:", contact)
		}
	}
	// The file is authoritative only for the address it names as its own.
	if got := fields["canonical"]; len(got) != 1 || got[0] != SiteURL+SecurityTxtPath {
		t.Errorf("Canonical is %q, want %s", got, SiteURL+SecurityTxtPath)
	}
}

func TestSecurityTxtExpiryIsMovedByAPerson(t *testing.T) {
	fields := securityTxtFields(t)
	if len(fields["expires"]) != 1 {
		t.Skip("the required-fields test already reports this")
	}
	expires, err := time.Parse(time.RFC3339, fields["expires"][0])
	if err != nil {
		t.Fatalf("Expires %q is not an RFC 3339 timestamp: %v", fields["expires"][0], err)
	}
	now := time.Now().UTC()
	switch {
	case !expires.After(now):
		t.Errorf("security.txt expired on %s; re-read the contacts and move the date", expires.Format(time.RFC3339))
	case expires.Sub(now) < expiryMargin:
		t.Errorf("security.txt expires on %s, within %s; re-read the contacts and move the date", expires.Format(time.RFC3339), expiryMargin)
	case expires.Sub(now) > maxLifetime:
		t.Errorf("security.txt expires on %s, more than %s away; RFC 9116 asks for less than a year", expires.Format(time.RFC3339), maxLifetime)
	}
}

// Exclusion requests and vulnerability reports are different queues.
func TestSecurityTxtDoesNotSendExclusionRequestsToSecurity(t *testing.T) {
	for _, contact := range securityTxtFields(t)["contact"] {
		if strings.Contains(strings.ToLower(contact), "abuse@") {
			t.Error("Contact names the abuse mailbox; exclusion requests and vulnerability reports are different queues")
		}
	}
}

func TestSecurityTxtPointsAtTheKey(t *testing.T) {
	encryption := securityTxtFields(t)["encryption"]
	if len(encryption) != 1 || encryption[0] != SiteURL+PGPKeyPath {
		t.Errorf("Encryption is %q, want %s, which is where the key is served", encryption, SiteURL+PGPKeyPath)
	}
}

func TestTheKeyIsAPublicKeyAndNothingMore(t *testing.T) {
	body := denyfirstFile(t, PGPKeyPath)
	for _, required := range []string{"-----BEGIN PGP PUBLIC KEY BLOCK-----", "-----END PGP PUBLIC KEY BLOCK-----"} {
		if !strings.Contains(body, required) {
			t.Errorf("the served file does not contain %q", required)
		}
	}
	if strings.Contains(body, "PRIVATE KEY") {
		t.Fatal("the served file contains a private key block")
	}
}

// The fingerprint is in three places, and two of them are behind different
// credentials: this site's file, and SECURITY.md on GitHub. A reporter
// compares them; taking one is not taking both.
func TestFingerprintAgreesAcrossSources(t *testing.T) {
	compact := strings.NewReplacer(" ", "", "\t", "").Replace
	if len(PGPFingerprint) != 40 || strings.Trim(PGPFingerprint, "0123456789ABCDEF") != "" {
		t.Fatalf("PGPFingerprint %q is not 40 uppercase hex digits", PGPFingerprint)
	}
	if !strings.Contains(compact(denyfirstFile(t, SecurityTxtPath)), PGPFingerprint) {
		t.Errorf("security.txt does not carry the fingerprint %s", PGPFingerprint)
	}
	policy, err := os.ReadFile("../../SECURITY.md")
	if err != nil {
		t.Fatalf("reading SECURITY.md: %v", err)
	}
	if !strings.Contains(compact(string(policy)), PGPFingerprint) {
		t.Errorf("SECURITY.md does not carry the fingerprint %s; a reporter has nothing to compare against", PGPFingerprint)
	}
}
