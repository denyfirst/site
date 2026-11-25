# Security policy

## Reporting a vulnerability

Please do not open a public issue for security problems.

**Preferred:** use GitHub's private vulnerability reporting — the *Security*
tab of this repository, then *Report a vulnerability*.

**Alternative:** email `security@denyfirst.dev`.
Reports may be encrypted. The key is at
[`https://denyfirst.dev/pgp-key.txt`](https://denyfirst.dev/pgp-key.txt) and
its fingerprint is:

    75B7 A18A 8971 5E37 75DB  CA2E A8D9 94D1 221A A045

This fingerprint is published in two places on purpose: here, and in
`security.txt` on the site. Whoever takes the domain serves their own key
beside their own fingerprint, and it would look exactly like the real one.
This repository is a different account behind different credentials, so
compare the two before encrypting anything.

The key certifies and encrypts and does nothing else. It is not the key that
signs commits; that one is an SSH key, listed in `.github/commit-signers`.

Please include enough detail to reproduce: what is affected, steps, and what an
attacker gains. Anonymous and pseudonymous reports are accepted without
question.

## What to expect

| Stage | Target |
|---|---|
| Acknowledgement | 72 hours |
| Initial assessment | 7 days |
| Fix or mitigation plan | 30 days for high and critical |

Coordinated disclosure, 90 days by default; earlier if a fix ships earlier,
and at once with whatever mitigation exists if the issue is being exploited.
Reporters are credited unless they ask not to be.

There is no bug bounty. This is an unfunded project and pretending otherwise
would waste your time.

## In scope

- The site at `denyfirst.dev` and the policy host `mta-sts.denyfirst.dev`
- Source code in this repository, and the workflows that test it

Of particular interest:

- Anything that makes a page load from, or tell anything to, another server
- Anything that writes a visitor's address, or anything about a visit, anywhere
- A redirect that can be steered off denyfirst.dev and porch.denyfirst.dev
- Content-Security-Policy bypass or injected markup
- A handshake answered for a name this server does not serve

This is the policy `security.txt` names, so it is where a report about
anything denyfirst publishes starts. Each product also has a policy of its own,
in its own repository, saying what is in scope there: Porch's is in
[denyfirst/porch](https://github.com/denyfirst/porch/blob/main/SECURITY.md).
The address and the key above take reports for any of them.

## Out of scope

- Missing headers with no demonstrated impact
- Automated scanner output submitted without verification
- Social engineering, physical access, or attacks on third-party infrastructure
- Denial of service through raw traffic volume

## Safe harbour

Good-faith research under this policy is authorised and will not be met with
legal action. Good faith means: do not access, modify, or exfiltrate other
people's data; do not degrade the service; stop as soon as you have confirmed
the issue; report it before disclosing it.

This covers the two names above and this repository only. It does not extend
to third-party services, and it does not override the law.
