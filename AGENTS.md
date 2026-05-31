# Agent Instructions

## Secret Handling

Do not read, print, cat, grep, parse, summarize, diff, or otherwise inspect
secret-bearing local files, including:

- `.env`
- `.env.*`
- `*.secret`
- `*.pem`
- `id_*`
- `*_token*`
- files or paths clearly containing credentials, private keys, access tokens, or
  personal secrets

If a task needs an environment variable such as `GITHUB_TOKEN`, use it only via
the process environment or Taskfile execution. Do not open the file that defines
the variable.

If secret contents appear in command output accidentally, do not repeat them in
the response.

## Project Workflow

This repository uses Task:

- `task test:smoke`
- `task test:smoke-prod`
- `task test:race-prod`
- `task version:get`
- `task version:tag`
- `task version:push-tag`

Use `GOCACHE=/tmp/go-build` for Go test commands when the default Go build
cache is not writable.
