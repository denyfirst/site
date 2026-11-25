# Cutting a release

A release is a signed list of hashes over a binary nobody built on the
signing machine. Two parties make it, and neither can do it alone:

| Step | Who | What it establishes |
|---|---|---|
| Signed tag | Maintainer | Which commit is released, and by whom |
| `build-release.yml` | A workflow that cannot sign | The binary is built from that commit in a public log, passes its tests and carries no known vulnerability |
| `release.ps1 -Compare` | Maintainer | The binary rebuilt on the signing machine is the same, byte for byte, and only then is the list signed |
| `reproduce.yml` | Another workflow | Someone other than the maintainer rebuilds the same bytes, and the release is signed |

`scripts/build.sh` is the only build command. Every step above runs it, and CI
fails if its flags appear anywhere else.

---

## The release

```powershell
git switch main
git pull

# Nothing may be waiting. Any output here is a stop: the build workflow
# refuses such a tag too, but by then the tag exists for ever.
gh pr list --state open

git log --oneline -1              # the commit this will release

git tag -s v0.1.0 -m "denyfirst.dev v0.1.0"

# The tag is new, and on the commit just read. Quoted, or PowerShell eats it.
git rev-parse "v0.1.0^{commit}"
git rev-parse main

git push origin v0.1.0
do { Start-Sleep 5; $id = gh run list --workflow=build-release.yml --limit 1 --branch v0.1.0 --json databaseId --jq '.[].databaseId' } until ($id)
gh run watch $id
```

The workflow builds, runs `go vet`, `go test` and `govulncheck` against the
tagged source, writes `SHA256SUMS` and `BUILD`, and leaves a **draft** release
nobody else can see.

### Sign

```powershell
git checkout v0.1.0
powershell.exe -ExecutionPolicy Bypass -File .\scripts\release.ps1 -Tag v0.1.0 -Compare
git switch main
```

`-Compare` rebuilds from the checked-out tag and compares. Expect the binary
to match, on Windows too. Any difference refuses the signature, and so does
anything else the script does not accept: it says why and signs nothing. Do
not work around it.

### Publish

Write the notes to `NOTES.md` outside the commit first, or the draft's "no
signature yet" text goes public. Then, as one block:

```powershell
$sig = 'dist\v0.1.0\SHA256SUMS.sig'; $got = Join-Path $env:TEMP ('sig-' + [guid]::NewGuid())
if (-not (Test-Path NOTES.md)) { Write-Host 'STOP: NOTES.md does not exist; nothing was uploaded' } elseif (Test-Path $sig) {
gh release upload v0.1.0 dist\v0.1.0\SHA256SUMS.sig --clobber
gh release edit v0.1.0 --notes-file NOTES.md
$body = (gh release view v0.1.0 --json body --jq .body) -join "`n"
$noted = ($body.Length -gt 0) -and ($body -notmatch 'no signature yet')
gh release download v0.1.0 --pattern SHA256SUMS.sig --dir $got
$ok = (Test-Path "$got\SHA256SUMS.sig") -and ((Get-FileHash "$got\SHA256SUMS.sig").Hash -eq (Get-FileHash $sig).Hash)
Remove-Item -Recurse -Force $got -ErrorAction SilentlyContinue
if ($ok -and $noted) {
gh release edit v0.1.0 --draft=false
Remove-Item NOTES.md
} elseif (-not $noted) { Write-Host 'STOP: the notes are not on the release; it stays a draft' } else { Write-Host 'STOP: the release does not carry the signature release.ps1 made; it stays a draft' }
} else { Write-Host "STOP: $sig does not exist, so release.ps1 signed nothing; nothing was uploaded" }
```

The block is one statement, so no failing step can be followed by the
publish, and the release is published only once the signature read back from
it is the one `release.ps1` wrote and the notes are not the draft's. Every
`elseif` and `else` stays on the line its brace closes: pasted into a
PowerShell window, an `elseif` on a line of its own is a command that does not
exist.

Publication starts `reproduce.yml`. Watch the run for this tag:

```powershell
do { Start-Sleep 5; $id = gh run list --workflow=reproduce.yml --limit 1 --branch v0.1.0 --json databaseId --jq '.[].databaseId' } until ($id)
gh run watch $id
```

---

## Repository settings this depends on

- **Merge commits only**, and signed commits on `main`: a rebase cannot keep a
  commit signature.
- **Tags cannot be deleted or moved.** Otherwise a signature over a tag says
  nothing about what was released.
