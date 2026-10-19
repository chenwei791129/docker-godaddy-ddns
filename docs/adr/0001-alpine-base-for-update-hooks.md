# Keep an Alpine base image so update hooks can run

The Success Hook and Failure Hook (`SUCCESS_EXEC` / `FAILED_EXEC`) are a
permanent feature, and they run arbitrary shell commands, so the image needs a
shell. We therefore build on Alpine instead of `scratch`, deliberately unlike
the sibling docker-cloudflare-ddns image, and accept a larger image and a wider
attack surface in exchange.

## Consequences

- Alpine package fixes reach users only through releases, so Dependabot bumps of
  the base and builder images use the `fix(deps):` commit type, which makes
  release-please cut a patch release for each of them.
- Switching to `scratch` or distroless would require dropping or replacing the
  hooks, which is a breaking change.
