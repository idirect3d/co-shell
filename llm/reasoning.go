package llm

import "strings"

// Response-field selectors for a model's thinking output (FEATURE-620).
//
// An OpenAI-compatible engine may report thinking under different JSON field
// names: vLLM used to emit "reasoning_content" and switched to "reasoning".
// The vendor documents the rename as a straight replacement and warns that a
// client still reading "reasoning_content" silently gets an empty string even
// when "reasoning" is populated. These constants let a model configuration
// state which field to read; the empty value means "auto" (read both).
const (
	// ReasoningFieldAuto reads either field, preferring "reasoning_content"
	// so that an unconfigured model behaves exactly as it did before.
	ReasoningFieldAuto = ""
	// ReasoningFieldContent reads only "reasoning_content" (legacy vLLM).
	ReasoningFieldContent = "reasoning_content"
	// ReasoningFieldReasoning reads only "reasoning" (current vLLM).
	ReasoningFieldReasoning = "reasoning"
	// ReasoningFieldInline means thinking is embedded in "content" (e.g.
	// wrapped in <thinking> tags) instead of a dedicated field.
	ReasoningFieldInline = "inline"
	// ReasoningFieldNone disables thinking extraction entirely.
	ReasoningFieldNone = "none"
)

// PickReasoning returns the thinking text carried by one response chunk.
//
// field is a ReasoningField* value. Auto — and any unrecognised value, so that
// a typo degrades to the safe behaviour — is the union fallback: the legacy
// "reasoning_content" wins when present, otherwise "reasoning" is used.
// "inline" deliberately returns nothing because inline thinking travels inside
// content and is handled by the content path.
func PickReasoning(field, reasoningContent, reasoning string) string {
	switch field {
	case ReasoningFieldContent:
		return reasoningContent
	case ReasoningFieldReasoning:
		return reasoning
	case ReasoningFieldInline, ReasoningFieldNone:
		return ""
	default: // ReasoningFieldAuto or unknown
		if reasoningContent != "" {
			return reasoningContent
		}
		return reasoning
	}
}

// ReasoningEchoField returns the JSON key under which an assistant message's
// thinking must be echoed back in a request. Only an explicit "reasoning"
// switches the key; auto and unknown values keep the legacy
// "reasoning_content" key so existing configurations are unaffected.
func ReasoningEchoField(field string) string {
	if field == ReasoningFieldReasoning {
		return ReasoningFieldReasoning
	}
	return ReasoningFieldContent
}

// InlineCoTStripper separates inline chain-of-thought from answer text
// (FEATURE-620).
//
// Some models wrap their reasoning inside content instead of reporting it in a
// dedicated field. Stripping that out must survive the text arriving in
// arbitrary streaming chunks, so the stripper buffers any trailing text that
// could still turn out to be the beginning of a tag: a tag fragment is never
// emitted as answer text, and no character is ever dropped.
//
// Stripping is off by default: it can only be enabled by an explicit
// ReasoningFieldInline plus a tag list, because a wrong tag list would cut real
// answer text out of the reply.
type InlineCoTStripper struct {
	tags []string
	// buf holds text whose meaning is not decided yet.
	buf string
	// open is the tag name currently open, empty while outside any tag.
	open string
}

// NewInlineCoTStripper builds a stripper for the given tag names. An empty tag
// list yields a stripper that passes everything through unchanged.
func NewInlineCoTStripper(tags []string) *InlineCoTStripper {
	return &InlineCoTStripper{tags: tags}
}

// Write consumes one content chunk and returns the answer text and the thinking
// text that are safe to emit now. Text that might still be part of a tag is
// held back until the next Write or until Flush.
func (s *InlineCoTStripper) Write(chunk string) (answer, thinking string) {
	if len(s.tags) == 0 {
		return chunk, ""
	}
	s.buf += chunk
	return s.drain(false)
}

// Flush returns whatever is still buffered. It must be called once the stream
// ends, so that a stream cut off in the middle of a tag neither loses text nor
// leaks a half tag into the answer.
func (s *InlineCoTStripper) Flush() (answer, thinking string) {
	if len(s.tags) == 0 {
		return "", ""
	}
	return s.drain(true)
}

// drain decides as much of the buffer as it can. When final is false, a
// trailing fragment that could still become a tag is kept back.
func (s *InlineCoTStripper) drain(final bool) (answer, thinking string) {
	for {
		if s.open == "" {
			idx, tag := earliestOpenTag(s.buf, s.tags)
			if idx < 0 {
				break
			}
			answer += s.buf[:idx]
			s.buf = s.buf[idx+len(tag):]
			s.open = tag[1 : len(tag)-1]
			continue
		}
		closing := "</" + s.open + ">"
		idx := strings.Index(s.buf, closing)
		if idx < 0 {
			break
		}
		thinking += s.buf[:idx]
		s.buf = s.buf[idx+len(closing):]
		s.open = ""
	}

	if final {
		rest := s.buf
		s.buf = ""
		// An unclosed tag means the stream was cut short: what follows the tag
		// is reasoning, and reporting it as such is better than dropping it or
		// leaking a stray tag as answer text.
		if s.open != "" {
			thinking += rest
		} else {
			answer += rest
		}
		return answer, thinking
	}

	// Hold back the longest suffix that could still become a tag.
	if s.open == "" {
		hold := 0
		for _, tag := range s.tags {
			if n := partialSuffixLen(s.buf, "<"+tag+">"); n > hold {
				hold = n
			}
		}
		answer += s.buf[:len(s.buf)-hold]
		s.buf = s.buf[len(s.buf)-hold:]
		return answer, thinking
	}
	hold := partialSuffixLen(s.buf, "</"+s.open+">")
	thinking += s.buf[:len(s.buf)-hold]
	s.buf = s.buf[len(s.buf)-hold:]
	return answer, thinking
}

// earliestOpenTag returns the position and literal of the earliest opening tag
// found in s, or -1 when there is none.
func earliestOpenTag(s string, tags []string) (int, string) {
	best := -1
	bestTag := ""
	for _, tag := range tags {
		literal := "<" + tag + ">"
		if i := strings.Index(s, literal); i >= 0 && (best < 0 || i < best) {
			best, bestTag = i, literal
		}
	}
	return best, bestTag
}

// partialSuffixLen returns the length of the longest proper prefix of literal
// that s currently ends with (0 when there is none). Such a suffix is withheld,
// because the next chunk may complete the literal.
func partialSuffixLen(s, literal string) int {
	max := len(literal) - 1
	if max > len(s) {
		max = len(s)
	}
	for n := max; n > 0; n-- {
		if strings.HasPrefix(literal, s[len(s)-n:]) {
			return n
		}
	}
	return 0
}
