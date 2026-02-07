package policy.assistant.v1

default allow_low_write := false
default require_approval_write := false

allow_low_write if write_action and risk_level == "low" and not prod_env and blast_radius != "account" and write_role_ok and not deny_write

require_approval_write if write_action and (prod_env or blast_radius == "account") and write_role_ok and not deny_write
require_approval_write if write_action and risk_level == "medium" and write_role_ok and not deny_write
require_approval_write if write_action and risk_level == "high" and write_role_ok and not deny_write

constraints["break_glass_required"] := true if write_action and risk_level == "high"
