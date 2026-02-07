package policy.assistant.v1

default decision := "deny"
default constraints := {}
default ttl := 0

# Decision selection: deny-by-default; explicit allow/read; explicit approval for writes.
decision := "allow" if allow_read else := "allow" if allow_low_write else := "require_approval" if require_approval_write else := "deny"
