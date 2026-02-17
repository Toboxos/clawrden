# Work In Progress

## Current Focus
- Setting up project foundation (Nix flake, Go module, directory structure)
- Implementing Phase 1 of the development roadmap

## Next Up
- Shared protocol package (`pkg/protocol`)
- Universal shim binary (`cmd/shim`)
- Warden core server (`cmd/warden`)

## Notes
- Target: Go 1.22+, statically compiled shim (CGO_ENABLED=0)
- Unix Domain Socket at `/var/run/clawrden/warden.sock`
- Length-prefixed JSON framing protocol
