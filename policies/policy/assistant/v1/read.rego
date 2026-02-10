package policy.assistant.v1

default allow_read := false

allow_read if {
	input.action.type == "read"
	authenticated_actor
}

ttl := 300 if {
	allow_read
}
