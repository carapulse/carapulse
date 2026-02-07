# OPA Policies

Files under `policies/` are an OPA bundle (see `policies/.manifest`).

## Local Test

```bash
opa test policies/ -v
```

## OPA API Contract

Carapulse queries:

- `POST /v1/data/policy/assistant/v1`
- body: `{"input":{...}}`
- expects: `{"result":{"decision":"allow|deny|require_approval","constraints":{...},"ttl":123}}`

