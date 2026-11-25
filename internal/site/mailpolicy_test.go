package site

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The policy, read the way RFC 8461 §3.2 has a sender read it: one key and
// value a line, a version first, a mode, the exchangers and a lifetime.
//
// Porch's mail check read our own policy and accepted it while one program
// served both; that check is Porch's now, and asks the live name. This reads
// the bytes before they are deployed.
func TestOurMailPolicyIsOneASenderAccepts(t *testing.T) {
	if !strings.HasPrefix(mailPolicy, "version: STSv1\r\n") {
		t.Error("the policy does not open with its version")
	}
	if !strings.HasSuffix(mailPolicy, "\r\n") {
		t.Error("the policy's last line is not ended")
	}
	fields := map[string][]string{}
	for _, line := range strings.Split(strings.TrimSuffix(mailPolicy, "\r\n"), "\r\n") {
		key, value, ok := strings.Cut(line, ": ")
		if !ok || strings.ContainsAny(line, "\r\n") || value == "" {
			t.Errorf("policy line %q is not one key and value", line)
			continue
		}
		fields[key] = append(fields[key], value)
	}
	if got := fields["mode"]; len(got) != 1 || got[0] != "testing" {
		t.Errorf("the policy's mode is %q; it moves to enforce on purpose, once the TLS reports are clean", got)
	}
	if got := fields["max_age"]; len(got) != 1 || got[0] != strconv.Itoa(7*24*60*60) {
		t.Errorf("the policy's max_age is %q, want one week while it is testing", got)
	}
	want := []string{"aspmx1.migadu.com", "aspmx2.migadu.com"}
	if got := fields["mx"]; !slices.Equal(got, want) {
		t.Errorf("the policy names %v; it should name the domain's two exchangers, %v, and nothing else", got, want)
	}
	for key := range fields {
		if !slices.Contains([]string{"version", "mode", "mx", "max_age"}, key) {
			t.Errorf("the policy carries %q, which RFC 8461 does not define", key)
		}
	}
}

// The policy's name answers the policy, and nothing else at all.
func TestThePolicyNameAnswersThePolicyAndNothingElse(t *testing.T) {
	for _, host := range []string{mailPolicyName, "MTA-STS.denyfirst.dev.", mailPolicyName + ":443"} {
		w := on(t, http.MethodGet, host, mailPolicyPath)
		if w.Code != http.StatusOK || w.Body.String() != mailPolicy {
			t.Errorf("GET %s%s = %d %q, want the policy", host, mailPolicyPath, w.Code, w.Body.String())
		}
		// RFC 8461 §3.3 has senders check for text/plain, and one that
		// compares the header as a string refuses a charset after it.
		if ct := w.Header().Get("Content-Type"); ct != "text/plain" {
			t.Errorf("the policy is served as %q", ct)
		}
	}

	w := on(t, http.MethodHead, mailPolicyName, mailPolicyPath)
	if w.Code != http.StatusOK || w.Body.Len() != 0 {
		t.Errorf("HEAD on the policy = %d with %d bytes, want 200 and no body", w.Code, w.Body.Len())
	}
	w = on(t, http.MethodPost, mailPolicyName, mailPolicyPath)
	if w.Code != http.StatusMethodNotAllowed || w.Header().Get("Allow") != "GET, HEAD" {
		t.Errorf("POST on the policy = %d, Allow %q; want 405 naming GET and HEAD", w.Code, w.Header().Get("Allow"))
	}

	// RFC 8461 §3.3 forbids a sender to follow a redirect, so nothing here is
	// one, and nothing of the site is reachable through this name.
	for _, target := range []string{"/", "/porch", "/privacy", "/organisation", "/api/v1/scan", "/healthz", SecurityTxtPath, "/security.txt", "/style.css", "/llms.txt", mailPolicyPath + "x", "/.well-known/"} {
		w := on(t, http.MethodGet, mailPolicyName, target)
		if w.Code != http.StatusNotFound {
			t.Errorf("GET %s on the policy name = %d, want 404", target, w.Code)
		}
		if loc := w.Header().Get("Location"); loc != "" {
			t.Errorf("GET %s on the policy name redirects to %s", target, loc)
		}
		if strings.Contains(w.Body.String(), "<html") || strings.Contains(w.Body.String(), "denyfirst") {
			t.Errorf("GET %s on the policy name reached the site behind it", target)
		}
	}
}

// And only the policy's name answers it.
func TestThePolicyIsServedOnlyAtItsOwnName(t *testing.T) {
	for _, host := range []string{siteName, "192.0.2.1", "localhost", "porch.denyfirst.dev"} {
		if w := on(t, http.MethodGet, host, mailPolicyPath); strings.Contains(w.Body.String(), "STSv1") {
			t.Errorf("%s answers %s with the policy; only %s should", host, mailPolicyPath, mailPolicyName)
		}
	}
}
