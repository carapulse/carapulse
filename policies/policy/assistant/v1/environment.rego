package policy.assistant.v1

default env_allowed := true

allowed_envs := {"local", "dev", "staging", "prod"}

env_allowed if env == ""
env_allowed if allowed_envs[env]

# Deny writes to unknown envs.
deny_write if write_action and not env_allowed

# Basic target limits.
deny_write if write_action and env == "prod" and input.risk.targets > 10
