package ai

import (
	"strings"
	"testing"
)

func TestTargetsHasFiveWithGroups(t *testing.T) {
	ts := targets()
	if len(ts) != 5 {
		t.Fatalf("targets() = %d, want 5", len(ts))
	}
	groups := map[string]int{}
	for _, tg := range ts {
		groups[tg.Group]++
	}
	if groups["claude"] != 3 {
		t.Fatalf("claude group count = %d, want 3 (CLAUDE.md + SKILL.md + project-context)", groups["claude"])
	}
	if groups["agents"] != 1 || groups["cursor"] != 1 {
		t.Fatalf("groups = %v, want agents=1 cursor=1", groups)
	}
}

func TestRenderAgentsAppendsWorkflow(t *testing.T) {
	body := renderAgents(renderInputs{LongBody: "# Design\n\narch body\n", WorkflowBody: "## Implementing a Feature with ncgo\nsteps\n"})
	if !strings.Contains(body, "arch body") || !strings.Contains(body, "Implementing a Feature with ncgo") {
		t.Fatalf("renderAgents missing long body or workflow:\n%s", body)
	}
}

func TestRenderCursorMDCUsesRulesBody(t *testing.T) {
	body := renderCursorMDC(renderInputs{RulesBody: "rule one\nrule two\n", LongBody: "full design doc\n"})
	if !strings.Contains(body, "rule one") || strings.Contains(body, "full design doc") {
		t.Fatalf("renderCursorMDC should embed rules not long body:\n%s", body)
	}
	if !strings.HasPrefix(body, "---\n") {
		t.Fatalf(".mdc must start with frontmatter:\n%s", body)
	}
}

func TestRenderNcgoDevSkillFrontmatterAndMarker(t *testing.T) {
	body := renderNcgoDevSkill(renderInputs{WorkflowBody: "## Implementing a Feature with ncgo\nsteps\n"})
	if !strings.HasPrefix(body, "---\nname: ncgo-dev\n") {
		t.Fatalf("SKILL.md must start with frontmatter name ncgo-dev:\n%s", body)
	}
	if !isManaged([]byte(body)) {
		t.Fatalf("SKILL.md must carry the managed marker within first 6 lines")
	}
	if !strings.Contains(body, "Implementing a Feature with ncgo") {
		t.Fatalf("SKILL.md missing workflow body:\n%s", body)
	}
}
