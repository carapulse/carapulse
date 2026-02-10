package policy.assistant.v1

break_glass := input.resources.break_glass == true

# Break-glass actions always require approval (and are annotated).
require_approval_write if {
	write_action
	tier == "break_glass"
	not deny_write
}

constraints["break_glass_used"] := true if {
	write_action
	tier == "break_glass"
	break_glass
}
