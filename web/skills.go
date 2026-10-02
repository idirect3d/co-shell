// Package web - Skills API (FEATURE-583, D-D).
//
// GET  /api/skills returns the discoverable skill list plus the "default
// enabled" switch, so the Skills settings page can render the list, the
// per-skill switches and the "follow default" state.
// POST /api/skills applies a change (default switch and/or per-skill toggles),
// persists the config and re-applies the agent skill state, then answers with
// the same payload as GET.
//
// The Server has no direct *agent.Agent pointer, so ServerOptions carries two
// callbacks (SkillsFn / SkillsSetFn) that bridge to the agent and the config.
// Both return an already-serialized JSON body which the handlers write out.
//
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/idirect3d/co-shell/agent"
)

// skillsPayload is the shared JSON body of GET and POST /api/skills.
type skillsPayload struct {
	DefaultEnabled bool              `json:"default_enabled"`
	Skills         []agent.SkillInfo `json:"skills"`
	// TotalBytes is the accumulated SKILL.md size of the currently loaded
	// skills (the same figure the skill_state event reports and the R2
	// "loaded skills" area shows). It is 0 when nothing is loaded.
	TotalBytes int `json:"total_bytes"`
}

// SkillsPayloadJSON builds the /api/skills response body. total_bytes counts
// only the loaded skills so it stays consistent with the skill_state event.
func SkillsPayloadJSON(defaultEnabled bool, skills []agent.SkillInfo) (string, error) {
	total := 0
	for _, s := range skills {
		if s.Loaded {
			total += s.Bytes
		}
	}
	data, err := json.Marshal(skillsPayload{
		DefaultEnabled: defaultEnabled,
		Skills:         skills,
		TotalBytes:     total,
	})
	return string(data), err
}

// ValidateSkillNames rejects names that were not discovered by the agent, so a
// POST body can never write an arbitrary key into skills_enabled.
func ValidateSkillNames(names map[string]bool, skills []agent.SkillInfo) error {
	if len(names) == 0 {
		return nil
	}
	known := make(map[string]bool, len(skills))
	for _, s := range skills {
		known[s.Name] = true
	}
	for name := range names {
		if !known[name] {
			return fmt.Errorf("unknown skill %q", name)
		}
	}
	return nil
}

// handleSkillsGet answers GET /api/skills.
func (s *Server) handleSkillsGet(w http.ResponseWriter, _ *http.Request) {
	if s.opts.SkillsFn == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "skills API unavailable"})
		return
	}
	data, err := s.opts.SkillsFn()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeRawJSON(w, data)
}

// handleSkillsSet answers POST /api/skills. The body is forwarded verbatim to
// SkillsSetFn, which validates it against the discovered skills, writes the
// config and re-applies the agent state.
func (s *Server) handleSkillsSet(w http.ResponseWriter, r *http.Request) {
	if s.opts.SkillsSetFn == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "skills API unavailable"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read body: " + err.Error()})
		return
	}
	data, err := s.opts.SkillsSetFn(body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeRawJSON(w, data)
}

// writeRawJSON writes an already-serialized JSON body with the JSON content
// type shared by every API response.
func writeRawJSON(w http.ResponseWriter, data string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, data)
}
