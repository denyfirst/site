# Verifying a release

Two questions are worth asking about the binary that serves denyfirst.dev, and
they are different questions.

**Did this come from the maintainer?** Answered by the signature.

**Does it correspond to the source you can read?** Answered by rebuilding it,
or by the workflow in this repository that rebuilds every release on a machine
the maintainer does not control.

Neither is answered by downloading over HTTPS. TLS tells you the file came
from GitHub, and GitHub is not the party you need to trust.

---

## Check the signature

Every release carries `SHA256SUMS`, the SHA-256 of each file, and
`SHA256SUMS.sig`, an OpenSSH signature over that list. One verification covers
every file, and the list is the record of what was released.

```sh
# The release you are checking, as its tag. Every command below reads it.
V=v0.1.0

# The public key that signs releases, from this repository.
curl -fsSLO https://raw.githubusercontent.com/denyfirst/site/main/.allowed_signers

# The list and its signature, from the release.
curl -fsSLO "https://github.com/denyfirst/site/releases/download/${V}/SHA256SUMS"
curl -fsSLO "https://github.com/denyfirst/site/releases/download/${V}/SHA256SUMS.sig"

ssh-keygen -Y verify -f .allowed_signers -I releases@denyfirst.dev \
  -n file -s SHA256SUMS.sig < SHA256SUMS
```

A pass prints:

```
Good "file" signature for releases@denyfirst.dev with ED25519 key SHA256:ut6bginhZ4lZINMSXNDv3vJ6fyvmDHhtnoBJH0/Nr9Y
```

Anything else means stop. The same key signs every denyfirst release; Porch's
`.allowed_signers` carries the same line, and comparing the two is a check
that does not rest on one repository.

## Check the binary against the list

```sh
curl -fsSLO "https://github.com/denyfirst/site/releases/download/${V}/denyfirstd_${V}_linux_amd64"
sha256sum --ignore-missing -c SHA256SUMS
```

## Rebuild it yourself

You need git, bash and Go; the `go` line in `go.mod` names the version, and Go
fetches it.

```sh
git clone https://github.com/denyfirst/site && cd site
git checkout "${V}"

# BUILD, from the release, records the Go version and the SHA-256 of the build
# script that ran. A different compiler or script gives different bytes from
# the same source: check both before concluding anything from a mismatch.
curl -fsSLO "https://github.com/denyfirst/site/releases/download/${V}/BUILD"
cat BUILD
sha256sum scripts/build.sh      # must match the buildscript line

bash scripts/build.sh "${V}" dist
sha256sum "dist/denyfirstd_${V}_linux_amd64"
```

The hash should be the one `SHA256SUMS` lists.

## Ask the binary what it is

```sh
./denyfirstd_${V}_linux_amd64 -version
```

It prints the tag it was built from.
