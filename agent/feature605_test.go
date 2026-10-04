// FEATURE-605 (agent side): the settings handler hot-applies a new sub-agent
// pool size to the running agent. This pins that the change reaches the
// <sub_agents max="N"> block injected into <environment_details>, and that sizes
// outside 0..MaxPoolSize are rejected.

package agent

import (
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/subagent"
)

// TestFeature605SetMaxSubAgentsHotApply covers UC-09: SetMaxSubAgents takes
// effect on the agent immediately (no restart) and the hard ceiling holds.
func TestFeature605SetMaxSubAgentsHotApply(t *testing.T) {
	a := &Agent{
		cfg: config.DefaultConfig(),
		subAgentMgr: subagent.NewManager(subagent.Options{
			Workspace:   t.TempDir(),
			NoStoreLoad: true,
		}),
	}
	a.SetSubAgentEnabled(true)

	if err := a.SetMaxSubAgents(2); err != nil {
		t.Fatalf("SetMaxSubAgents(2) = %v", err)
	}
	if got := a.subAgentBlock(); !strings.Contains(got, `max="2"`) {
		t.Errorf("subAgentBlock() = %q, want max=\"2\"", got)
	}

	// 0 is legal (creation disabled) and the block still reports the cap, which
	// is what tells the model no id is available.
	if err := a.SetMaxSubAgents(0); err != nil {
		t.Fatalf("SetMaxSubAgents(0) = %v", err)
	}
	if got := a.subAgentBlock(); !strings.Contains(got, `max="0"`) {
		t.Errorf("subAgentBlock() after 0 = %q, want max=\"0\"", got)
	}

	// The hard ceiling is accepted, anything beyond it is not.
	if err := a.SetMaxSubAgents(subagent.MaxPoolSize); err != nil {
		t.Errorf("SetMaxSubAgents(%d) = %v, want nil", subagent.MaxPoolSize, err)
	}
	if err := a.SetMaxSubAgents(subagent.MaxPoolSize + 1); err == nil {
		t.Error("SetMaxSubAgents(MaxPoolSize+1) must be rejected")
	}
	if got := a.subAgentBlock(); !strings.Contains(got, `max="10"`) {
		t.Errorf("a rejected size must leave the pool at 10, block = %q", got)
	}
}
