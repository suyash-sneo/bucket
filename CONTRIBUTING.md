# Contributing to bucket

Thanks for your interest in contributing.

## Ways to contribute

- Report bugs
- Suggest features or UX improvements
- Submit pull requests

## Report an issue

Please open a GitHub Issue and include:

- What happened
- What you expected
- Steps to reproduce
- OS + terminal details
- App version (`bucket --version` if available)
- Relevant logs from `~/.config/bucket/log.txt`

## Suggest a feature

Open a GitHub Issue with:

- Problem statement (what friction you are facing)
- Proposed behavior
- Why it fits bucket’s keyboard-first workflow

## Submit a pull request

1. Fork the repository
2. Create a feature branch
3. Make focused changes
4. Run:

```sh
go test ./...
go build ./cmd/bucket
```

5. Open a PR with:
   - Summary of changes
   - Screenshots/GIFs for TUI changes (if applicable)
   - Linked issue number(s)

## Style expectations

- Keep changes minimal and scoped
- Preserve keyboard-first behavior
- Avoid unrelated refactors in the same PR
- Add/update tests for behavior changes when possible

## Security issues

If you find a security issue, please avoid posting exploit details publicly in an issue.
Open an issue with minimal details and request a private contact channel for disclosure.
