package auth

type Admin struct {
	ID       string
	Password string
}

type Principal struct {
	UserID       string   `json:"user_id"`
	Roles        []string `json:"roles"`
	Capabilities []string `json:"capabilities"`
}

func (p *Principal) HasCapability(capability string) bool {
	if p == nil || capability == "" {
		return false
	}

	for _, current := range p.Capabilities {
		if current == capability {
			return true
		}
	}

	return false
}
