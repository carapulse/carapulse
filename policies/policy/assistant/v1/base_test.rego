package policy.assistant.v1

test_read_allows if {
	inp := {"actor": {"id": "a", "roles": ["viewer"]}, "action": {"type": "read"}, "risk": {"level": "read"}}
	decision with input as inp == "allow"
	ttl with input as inp == 300
}

test_write_low_nonprod_allows if {
	inp := {"actor": {"id": "a", "roles": ["operator"]}, "action": {"type": "write"}, "context": {"environment": "dev"}, "risk": {"level": "low", "targets": 1, "blast_radius": "service"}}
	decision with input as inp == "allow"
}

test_write_medium_requires_approval if {
	inp := {"actor": {"id": "a", "roles": ["operator"]}, "action": {"type": "write"}, "context": {"environment": "dev"}, "risk": {"level": "medium", "targets": 1, "blast_radius": "service"}}
	decision with input as inp == "require_approval"
}

test_write_high_requires_approval_and_break_glass_flag if {
	inp := {"actor": {"id": "a", "roles": ["operator"]}, "action": {"type": "write"}, "context": {"environment": "dev"}, "risk": {"level": "high", "targets": 1, "blast_radius": "service"}}
	decision with input as inp == "require_approval"
	bg := constraints.break_glass_required with input as inp
	bg == true
}

test_write_prod_requires_approval if {
	inp := {"actor": {"id": "a", "roles": ["operator"]}, "action": {"type": "write"}, "context": {"environment": "prod"}, "risk": {"level": "low", "targets": 1, "blast_radius": "service"}}
	decision with input as inp == "require_approval"
}

test_write_account_requires_approval if {
	inp := {"actor": {"id": "a", "roles": ["operator"]}, "action": {"type": "write"}, "context": {"environment": "dev"}, "risk": {"level": "low", "targets": 1, "blast_radius": "account"}}
	decision with input as inp == "require_approval"
}

test_write_no_roles_denies if {
	inp := {"actor": {"id": "a", "roles": []}, "action": {"type": "write"}, "context": {"environment": "dev"}, "risk": {"level": "low", "targets": 1, "blast_radius": "service"}}
	decision with input as inp == "deny"
}

test_write_unknown_env_denies if {
	inp := {"actor": {"id": "a", "roles": ["operator"]}, "action": {"type": "write"}, "context": {"environment": "nope"}, "risk": {"level": "low", "targets": 1, "blast_radius": "service"}}
	decision with input as inp == "deny"
}

