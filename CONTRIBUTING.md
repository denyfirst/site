# Contributing to denyfirst.dev

These are the rules the code does not show. The reasons for each guard are in
the comments of the tests that hold it.

---

## What this is

The organisation's site and its mail policy host, served by one small program
built from Go's standard library alone. **No third-party dependencies:**
`go.mod` has no `require` block, and it stays that way.

The map, which CI holds to the tree:

| Package | What it is for |
|---|---|
| `internal/site` | The pages, redirects, mail policy and crawler files, embedded and served from a table |
| `internal/promises` | The organisation's undertakings, and the products this site links to |

`cmd/denyfirstd` is the server: TLS, the certificate's names, the timeouts and
the connection limit.

---

## The three priorities, in order

Security, privacy, anonymity: of this site and of whoever reads it. A change
that improves one at the cost of another is argued before it is written.

- **Record nothing.** No log of visits, no address, no time.
- **Load nothing from elsewhere.** Every byte a page needs is served from here,
  and the Content-Security-Policy says so where a browser enforces it.
- **Say only what can be checked.** A sentence on a page that states a fact
  has a test that holds it to the thing it describes.
- **Keep nothing that is not used.** A file, a rule or a line of style nothing
  needs is weight nobody reviews; the tests refuse an embedded file nothing
  serves and a style for something no page has.

---

## Rules for a change

- **Sabotage what you wrote, in both directions.** Break the new behaviour and
  check a test fails; break it the other way and check again.
- **A page's address does not move.** If one must, the old address answers with
  a permanent redirect, and a test names it.
- **The redirects to Porch stay.** Every address of Porch's that denyfirst.dev
  ever answered is in a link somebody kept.

---

## Rules for git

- **Commit messages carry the reason for the change and nothing else.** No
  attribution trailers, no session, tool or account identifiers, no link a
  reader of this repository cannot follow.
- **Every commit is signed** by a key in `.github/commit-signers`. CI checks
  each commit of a pull request.
- **Stage by name, never `git add -A`,** and read the index before and after.
- **Merge commits only.** A rebase cannot keep a commit signature.
- **One build command.** `scripts/build.sh` decides a release's bytes, and
  CI fails if its flags appear anywhere else. Cutting a release is
  `docs/releasing.md`.
- **Nothing that describes the server** — paths, accounts, unit names,
  monitoring, hardening — is written here. It lives in the private operations
  repository.

---

## Running the gates

```sh
go mod tidy && git status --porcelain   # nothing
go vet ./...
gofmt -l .                              # nothing
go test -race -shuffle=on ./...
```

The front page states facts about Porch's repository — its key, its licence,
its release steps — and one test reads them there:

```sh
git clone --depth 1 https://github.com/denyfirst/porch ../porch
PORCH_CHECKOUT=../porch go test -run FactsAboutPorch -v ./internal/site/
```
