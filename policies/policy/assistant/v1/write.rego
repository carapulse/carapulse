package policy.assistant.v1

default allow_low_write := false
default require_approval_write := false

allow_low_write if {
	write_action
	risk_level == "low"
	not prod_env
	blast_radius != "account"
	write_role_ok
	not deny_write
}

require_approval_write if {
	write_action
	prod_env
	write_role_ok
	not deny_write
}

require_approval_write if {
	write_action
	blast_radius == "account"
	write_role_ok
	not deny_write
}

require_approval_write if {
	write_action
	risk_level == "medium"
	write_role_ok
	not deny_write
}

require_approval_write if {
	write_action
	risk_level == "high"
	write_role_ok
	not deny_write
}

constraints["break_glass_required"] := true if {
	write_action
	risk_level == "high"
}
