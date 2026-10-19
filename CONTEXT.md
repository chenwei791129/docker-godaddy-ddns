# GoDaddy DDNS

Keeps one GoDaddy-hosted DNS record pointing at the public IPv4 address of the
machine the container runs on.

## Language

### Credentials

**Personal Access Token (PAT)**:
GoDaddy's current scoped credential, sent as a bearer token; the preferred way
to authenticate.
_Avoid_: API token, bearer key

**Classic Developer Key**:
GoDaddy's legacy key-and-secret credential pair, which GoDaddy has deprecated
for the Domains API.
_Avoid_: API key, sso-key, legacy key

### Records

**Managed Record**:
The single DNS record this service owns, identified by domain, record name and
record type; only IPv4 address (`A`) records are supported.
_Avoid_: DNS entry, host record

**Public IP**:
The IPv4 address the internet sees for the machine running the service, as
reported by the configured IP check service.
_Avoid_: external IP, WAN IP, current IP

### Hooks

**Success Hook**:
A user-supplied shell command run after the Managed Record was changed to a new
Public IP; never run when the record was already up to date.
_Avoid_: callback, post-update script

**Failure Hook**:
A user-supplied shell command run when the Public IP cannot be determined or the
Managed Record cannot be updated.
_Avoid_: error callback, on-error script

### Compatibility

**Deprecated Setting**:
A configuration option that still works in the current major version but logs a
warning and is scheduled for removal in the next major version.
_Avoid_: legacy option, old variable
