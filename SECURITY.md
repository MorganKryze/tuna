# Security policy

## Supported versions

The latest release. Older tags stay downloadable but are not patched.

## Reporting a vulnerability

Use GitHub's private reporting: **Security → Report a vulnerability** on this
repository. Please do not open a public issue for anything you believe is
exploitable.

You can expect an acknowledgement within a week. If the report is confirmed,
the fix lands in a patch release and the advisory is published once the release
is out.

## Verifying what you downloaded

Release binaries are built by this repository's CI and published with a
`SHA256SUMS` next to them:

```sh
sha256sum -c SHA256SUMS --ignore-missing
```

They also carry a build attestation, which is what ties an artifact to the
workflow that produced it rather than to whoever uploaded it:

```sh
gh attestation verify tunny-darwin-arm64 --repo MorganKryze/tunny
```

CI runs `govulncheck` weekly, which is what surfaces a CVE in a dependency
without waiting for the next commit.

## Scope worth knowing

tunny's attack surface is small on purpose: it makes no network connection of
its own, listens on nothing, and stores no secret. It reads one TOML file,
writes one file of names, and executes `ssh` with an argument list. Everything
that authenticates belongs to ssh and to your `~/.ssh/config`: keys, the agent,
`known_hosts`, the host-key prompt. That is why tunny delegates instead of
reimplementing any of it.

The interesting reports are therefore about the argument list: a value in
`destinations.toml` that turns into an ssh option it should not, or a `host`
that ends up read as something other than a target. Note that
`destinations.toml` is trusted input — it is your own file — so "a malicious config
can do X" only matters if X escapes what editing that file could already do.

One operational note rather than a vulnerability: `destinations.toml` describes
internal addresses and non-standard ssh ports. Keep it in `~/.config`, out of
any repository that might become public. The `destinations.example.toml` shipped
here carries generic values for exactly that reason.
