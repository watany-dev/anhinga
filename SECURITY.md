# Security Policy

## Supported Versions

Only the latest release of `anhinga` is supported. Fixes are shipped in a new
release rather than backported.

## Reporting a Vulnerability

Please **do not** open a public issue for a security problem.

Report it privately through GitHub Security Advisories:
[Report a vulnerability](https://github.com/watany-dev/anhinga/security/advisories/new)

Include, as far as you can:

- what the issue is and how it can be triggered
- the version or commit you tested
- the impact you believe it has

You can expect an initial response within about a week. Once a fix is ready
we will publish it in a release along with an advisory crediting you, unless
you prefer otherwise.

## Scope

`anhinga` is a read-only CLI: it calls the EC2 and CloudTrail APIs with the
caller's own AWS credentials and prints the result. Reports about credential
handling, the release artifacts, or the install script are in scope. AWS
service behaviour itself is not — please report that to AWS.

## Automated checks

Every pull request runs golangci-lint (including gosec), semgrep, govulncheck
and gitleaks. See `.github/workflows/`.
