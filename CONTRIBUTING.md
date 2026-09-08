# Contributing

Contributions are welcome through GitHub issues and pull requests.

## Development

1. Fork and clone the repository.
2. Create a focused branch.
3. Make the smallest change that addresses the issue.
4. Add deterministic tests. Tests must not require Google credentials or make
   live Google API calls.
5. Run:

   ```sh
   go mod tidy
   go build ./...
   go vet ./...
   go test -race ./...
   ```

6. Open a pull request describing the behavior changed and the checks run.

Never commit OAuth client secrets, access tokens, refresh tokens, user data, or
generated credential files. By contributing, you agree that your contribution
is licensed under the repository's MIT license.
