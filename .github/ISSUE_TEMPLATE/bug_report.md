---
name: Bug report
about: A tunnel that will not open, will not close, or will not stay up
labels: bug
---

**What happened?**

**What did you expect?**

**Your setup**: `tunny` version, operating system, and the relevant
`[[destination]]` block with the hostnames redacted.

**What does ssh say?** Run the same thing by hand with `-v` and paste the
output. Without it, "it does not connect" is not diagnosable:

```sh
ssh -v -N -o ExitOnForwardFailure=yes -L <local>:<to> <host>
```
