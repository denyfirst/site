# denyfirst.dev

The organisation's site: its front page, what it promises for everything it
publishes, and where to report a security problem. Also
`mta-sts.denyfirst.dev`, which serves the policy for mail to denyfirst.dev and
nothing else.

Each product has a site of its own. Porch is at
[porch.denyfirst.dev](https://porch.denyfirst.dev/), from
[denyfirst/porch](https://github.com/denyfirst/porch); every address of
Porch's that denyfirst.dev answered before 2026-11 answers here with a
permanent redirect to the same path there.

## What it is made of

- `cmd/denyfirstd` — the server. TLS only, HTTP/1.1 only, and a handshake for
  any name but the two it serves is refused before a certificate is sent.
- `internal/site` — the pages, the redirects, the mail policy and the crawler
  files. Everything is embedded in the binary and served from a fixed table.
- `internal/promises` — what denyfirst undertakes as an organisation, and the
  products that add their own undertakings to it.

Go's standard library and nothing else: `go.mod` has no `require` block.

The pages set no cookie, load nothing from another server, carry no tracker,
and keep nothing about a visit. The server writes no log of visits; the one
line it prints names the program, not a request.

## Running it

```sh
go test ./...
go build -trimpath -o denyfirstd ./cmd/denyfirstd
./denyfirstd -listen 127.0.0.1:8443 -tls-cert cert.pem -tls-key key.pem
```

It answers over TLS only, so it needs a certificate covering `denyfirst.dev`
and `mta-sts.denyfirst.dev`; a self-signed one does for a local look, with
`curl --resolve denyfirst.dev:8443:127.0.0.1 --cacert ca.pem`.

## Releases

Each release is built from a signed tag by a public workflow that cannot sign,
rebuilt on the maintainer's machine and signed only if the bytes match, and
rebuilt again by a second workflow after it is published. The key that signs
it is in [`.allowed_signers`](.allowed_signers), the same key as every
denyfirst release.

- Checking one: [docs/verify.md](docs/verify.md)
- Cutting one: [docs/releasing.md](docs/releasing.md)

## Licence

AGPL-3.0. See [LICENSE](LICENSE).
