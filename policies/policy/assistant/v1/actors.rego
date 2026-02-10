package policy.assistant.v1

default authenticated_actor := false
default write_role_ok := false

actor_id := object.get(input.actor, "id", object.get(input.actor, "ID", ""))
roles := object.get(input.actor, "roles", object.get(input.actor, "Roles", []))

authenticated_actor if {
	actor_id != ""
}

has_role(r) if {
	roles[_] == r
}

write_role_ok if {
	has_role("admin")
}

write_role_ok if {
	has_role("operator")
}

# Deny if no roles (write only; reads handled separately).
deny_write if {
	write_action
	not write_role_ok
}
