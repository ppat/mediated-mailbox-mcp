# Mutations: cli

The demonstrations of the controls whose patches sit in `cli/`. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## The key-generation command writes a pair the library seals to and opens with

- **Date · evidence:** 2026-11-19 · [pull request #478](https://github.com/ppat/mediated-mailbox-mcp/pull/478), in run [44442809114](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/44442809114) of the `mutation-demonstrations` workflow
- **Break (1):** an existing key file is truncated and overwritten rather than refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/cli/internal/keygen`:** `TestTheCommandPrintsTheKeyAndOverwritesNothing`
- **Break (2):** the mode is left to the process's umask, so a restrictive umask narrows the public key file's mode
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/cli/internal/keygen`:** `TestTheKeyFilesModesHoldUnderAnyUmask`
- **Break (3):** the private key file is written with mode 0644, readable by every user
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/cli/internal/keygen`:** `TestTheKeyFilesModesHoldUnderAnyUmask`
- **Break (4):** the public key file holds another pair's public key
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/cli`:** `TestTheCommandWritesAPairTheLibrarySealsToAndOpensWith`
- **Break (5):** the private key file holds the seed followed by a newline
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/cli`:** `TestTheCommandWritesAPairTheLibrarySealsToAndOpensWith`
