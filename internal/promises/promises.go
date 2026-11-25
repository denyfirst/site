// Package promises holds what denyfirst undertakes as an organisation, and
// the products that add their own undertakings to it.
//
// The organisation's list is about its own conduct and names no product. A
// product adds narrower undertakings of its own and never rewords these. Each
// product's own list lives with that product's source, where the code that
// keeps it is; what this site carries of a product is its name, one line on
// what it is for, and the address of the page where its undertakings are.
package promises

// Promise is one undertaking, and how somebody can check it rather than believe
// it.
type Promise struct {
	// ID is a stable identifier, used to say which undertaking is meant
	// without quoting it. It outlives rewording: the sentence may be improved,
	// and anything referring to it still refers to the same undertaking.
	ID string

	// Says is the undertaking, in the words it is made in.
	Says string

	// Checked is how somebody establishes it for themselves.
	//
	// Required, and the reason it is required is the whole argument of this
	// package: an undertaking nobody can check is a request to be trusted, and
	// a request to be trusted is what this organisation is trying not to make.
	Checked string
}

// Organisation is what denyfirst undertakes, whatever you run.
var Organisation = []Promise{
	{
		ID:      "nothing-reaches-us",
		Says:    "Nothing we publish reports back to us: no telemetry, no crash reports, no licence checks. A tool of ours looks for a newer release only when you ask it to, or turn that on, and then asks the public host the release is on, never us, sending nothing about you.",
		Checked: "The source is public. It contains no address of ours that a program connects to; the only one it asks about releases is GitHub's, and the request carries no identifier and no version.",
	},
	{
		ID:      "no-register-of-users",
		Says:    "You need no account with us, and we keep no list of who runs our tools.",
		Checked: "There is nothing to sign up for. A release is a set of files and a signature.",
	},
	{
		ID:      "nothing-from-third-parties",
		Says:    "Our pages load nothing from anywhere else: no analytics, no external fonts, no CDN, no tags.",
		Checked: "Your browser's network panel shows every request going only to the server you are reading.",
	},
	{
		ID:      "checkable-rather-than-believed",
		Says:    "You can check what we publish instead of trusting it. The source is public, releases are signed, and anyone can rebuild a release to the same bytes.",
		Checked: "Each product publishes its release steps with its source. A public workflow rebuilds every release and compares the hashes.",
	},
	{
		ID:      "a-product-may-only-promise-less",
		Says:    "Where we run a service, it says exactly what it keeps. A product may keep less than this list allows, never more, and may not reword these.",
		Checked: "Each product lists its additions on its own page. The texts are written once in the source, and a test fails if a product rewords one of these.",
	},
	{
		ID:      "faults-have-an-address",
		Says:    "Security problems can be reported to a published address that says when it expires.",
		Checked: "https://denyfirst.dev/.well-known/security.txt (RFC 9116) names the contact and the expiry date.",
	},
}

// Product is one thing denyfirst publishes, as this site names it.
type Product struct {
	// Name is what the product is called.
	Name string

	// What is one line saying what it is for, so that a reader of the
	// organisation's page knows which product a restriction belongs to.
	What string

	// Home is the product's own address, where its own llms.txt is.
	Home string

	// Promises is the page that says what the product adds to the list
	// above, at the anchor of that list.
	Promises string
}

// Products is every product this site links to.
//
// A product is listed once it has a page that says what it adds. One that is
// being built is in Coming instead: a link to a page that does not exist yet
// would send a reader to a promise nobody has made.
var Products = []Product{
	{
		Name:     "Porch",
		What:     "Checks TLS, websites, mail and DNS, and grades what it finds.",
		Home:     "https://porch.denyfirst.dev/",
		Promises: "https://porch.denyfirst.dev/privacy#promises",
	},
}

// Coming is what denyfirst is building and has not released, in the order of
// their names. Each is named with what it is for and nothing more: no address,
// because there is no page to send a reader to, and no date, because none is
// promised. The menu, the front page and llms.txt are all drawn from this
// list, so a product announced in one is announced in all three.
//
// A product moves to Products on the day it has a page saying what it adds.
var Coming = []Product{
	{
		Name: "Keybash",
		What: "A self-hosted workspace for SSH and RDP: your own credentials, and your servers from a browser.",
	},
	{
		Name: "Mohur",
		What: "Private messages between organisations that each run their own server, end-to-end encrypted, with no central service.",
	},
	{
		Name: "Rootwell",
		What: "A self-hosted workspace for certificates and keys.",
	},
}
