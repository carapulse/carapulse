package policy

type Action struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type Risk struct {
	Level       string `json:"level"`
	Targets     int    `json:"targets,omitempty"`
	BlastRadius string `json:"blast_radius,omitempty"`
	Tier        string `json:"tier,omitempty"`
}
