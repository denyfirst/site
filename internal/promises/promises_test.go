package promises

import (
	"net/url"
	"regexp"
	"strings"
	"testing"
)

var stableID = regexp.MustCompile(`^[a-z]+(-[a-z]+)*$`)

// Every promise has a stable name, says something, and says how it is
// checked: an undertaking nobody can check is a request to be trusted.
func TestEveryPromiseSaysHowItIsChecked(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range Organisation {
		if !stableID.MatchString(p.ID) {
			t.Errorf("%q is not a stable name of lowercase words", p.ID)
		}
		if seen[p.ID] {
			t.Errorf("two promises are %s", p.ID)
		}
		seen[p.ID] = true
		if strings.TrimSpace(p.Says) == "" || strings.TrimSpace(p.Checked) == "" {
			t.Errorf("%s says nothing, or nothing about how it is checked", p.ID)
		}
		if !strings.HasSuffix(p.Says, ".") || !strings.HasSuffix(p.Checked, ".") {
			t.Errorf("%s is not written in sentences", p.ID)
		}
	}
	if len(Organisation) != 6 {
		t.Errorf("the organisation makes %d promises; a change to that is a change of what it undertakes, and this number moves with it", len(Organisation))
	}
}

// The organisation's list names no product: each product's additions are its
// own, on its own page.
func TestTheOrganisationsPromisesNameNoProduct(t *testing.T) {
	for _, p := range Organisation {
		for _, product := range Products {
			if strings.Contains(p.Says, product.Name) || strings.Contains(p.Checked, product.Name) {
				t.Errorf("%s names %s", p.ID, product.Name)
			}
		}
	}
}

// A product's addresses are on its own name under denyfirst.dev, over TLS,
// and its promises are at an anchor.
func TestEveryProductHasItsOwnAddress(t *testing.T) {
	if len(Products) == 0 {
		t.Fatal("no product is listed")
	}
	for _, p := range Products {
		if p.Name == "" || p.What == "" || !strings.HasSuffix(p.What, ".") {
			t.Errorf("%q is not named and described in a sentence", p.Name)
		}
		home, err := url.Parse(p.Home)
		if err != nil || home.Scheme != "https" || !strings.HasSuffix(home.Host, ".denyfirst.dev") || home.Path != "/" {
			t.Errorf("%s's home %q is not the root of its own name under denyfirst.dev", p.Name, p.Home)
		}
		promises, err := url.Parse(p.Promises)
		if err != nil || promises.Scheme != "https" || promises.Host != home.Host || promises.Fragment == "" {
			t.Errorf("%s's promises %q are not at an anchor on its own name", p.Name, p.Promises)
		}
	}
}
