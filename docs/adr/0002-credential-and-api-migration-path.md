# Migrate credentials and the GoDaddy API across major versions

GoDaddy now prefers a Personal Access Token (PAT) and has deprecated the Classic
Developer Key for the Domains API, while the v1 DNS endpoints this service uses
accept both. Version 1.x therefore stays on the v1 endpoints and accepts either
credential (a PAT wins when both are set), so existing users can upgrade
without changing their configuration. Version 2.0.0 drops the Classic Developer
Key and moves to the v3 Domains API, which only accepts a PAT.

## Consequences

- In 1.x the Classic Developer Key, `CRON_TIME`, the `alpine` image tag and the
  `PublicIP` hook variable are Deprecated Settings: they keep working and log or
  document a warning.
- 2.0.0 removes all of them. v3 replaces records by record ID, so the GoDaddy
  client has to look a record up before replacing it; keeping the client
  behind its own package limits that change to one place.

## Considered Options

- Using v3 for PATs and v1 for Classic Developer Keys in 1.x was rejected: it
  doubles the API code paths for a transition that 2.0.0 completes anyway.
