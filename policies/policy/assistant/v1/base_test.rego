package policy.assistant.v1

test_read_allows if {
  input := {"actor":{"id":"a","roles":["viewer"]},"action":{"type":"read"},"risk":{"level":"read"}}
  decision with input as input == "allow"
  ttl with input as input == 300
}

test_write_low_nonprod_allows if {
  input := {"actor":{"id":"a","roles":["operator"]},"action":{"type":"write"},"context":{"environment":"dev"},"risk":{"level":"low","targets":1,"blast_radius":"service"}}
  decision with input as input == "allow"
}

test_write_medium_requires_approval if {
  input := {"actor":{"id":"a","roles":["operator"]},"action":{"type":"write"},"context":{"environment":"dev"},"risk":{"level":"medium","targets":1,"blast_radius":"service"}}
  decision with input as input == "require_approval"
}

test_write_high_requires_approval_and_break_glass_flag if {
  input := {"actor":{"id":"a","roles":["operator"]},"action":{"type":"write"},"context":{"environment":"dev"},"risk":{"level":"high","targets":1,"blast_radius":"service"}}
  decision with input as input == "require_approval"
  constraints.break_glass_required with input as input == true
}

test_write_prod_requires_approval if {
  input := {"actor":{"id":"a","roles":["operator"]},"action":{"type":"write"},"context":{"environment":"prod"},"risk":{"level":"low","targets":1,"blast_radius":"service"}}
  decision with input as input == "require_approval"
}

test_write_account_requires_approval if {
  input := {"actor":{"id":"a","roles":["operator"]},"action":{"type":"write"},"context":{"environment":"dev"},"risk":{"level":"low","targets":1,"blast_radius":"account"}}
  decision with input as input == "require_approval"
}

test_write_no_roles_denies if {
  input := {"actor":{"id":"a","roles":[]},"action":{"type":"write"},"context":{"environment":"dev"},"risk":{"level":"low","targets":1,"blast_radius":"service"}}
  decision with input as input == "deny"
}

test_write_unknown_env_denies if {
  input := {"actor":{"id":"a","roles":["operator"]},"action":{"type":"write"},"context":{"environment":"nope"},"risk":{"level":"low","targets":1,"blast_radius":"service"}}
  decision with input as input == "deny"
}

