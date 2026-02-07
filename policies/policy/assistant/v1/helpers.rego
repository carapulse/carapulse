package policy.assistant.v1

default deny_write := false

write_action := input.action.type == "write"

risk_level := object.get(input.risk, "level", object.get(input.risk, "Level", ""))
blast_radius := object.get(input.risk, "blast_radius", object.get(input.risk, "BlastRadius", ""))
tier := object.get(input.risk, "tier", object.get(input.risk, "Tier", ""))

env := lower(object.get(input.context, "environment", object.get(input.context, "Environment", "")))
prod_env := env == "prod"
