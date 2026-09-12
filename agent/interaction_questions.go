// Package agent - multi-question interaction (FEATURE-512).
//
// This file implements the terminal rendering of the "questions" interaction
// kind plus the answer formatting shared with the ask_user tool. The interaction
// model itself lives in interaction.go.
//
// Author: L.Shuang
// Created: 2026-09-13
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/i18n"
)

// askQuestions renders a multi-question form one question at a time and collects
// every answer in a single round (FEATURE-512). Per question the user may pick
// option numbers (comma separated when multi-choice), type text directly, or
// prefix the input with a space to leave a free-form note. The fixed keys
// carried by the interaction are honoured on every question: the think_exit key
// aborts the whole form, any other key answers the current question with its
// label. Empty input leaves the question unanswered, which is allowed.
func (m *TerminalInteractionManager) askQuestions(in Interaction) (InteractionResult, error) {
	answers := make([]QuestionAnswer, 0, len(in.Questions))
	total := len(in.Questions)

	for i, q := range in.Questions {
		m.renderQuestion(i+1, total, q, in.Keys)
		answer, cancelled, err := m.readQuestionAnswer(q, in.Keys)
		if err != nil {
			return InteractionResult{}, err
		}
		if cancelled {
			return InteractionResult{Action: ActionCancel}, nil
		}
		answers = append(answers, answer)
	}

	m.io.Println()
	return InteractionResult{Action: ActionSubmit, Answers: answers}, nil
}

// renderQuestion prints one question with its options and fixed keys.
func (m *TerminalInteractionManager) renderQuestion(idx, total int, q Question, keys []KeyOption) {
	m.io.Println()
	m.io.Printf(i18n.TF(i18n.KeyAskUserQuestionLabel, idx, total, q.Title))
	m.io.Println()
	if len(q.Options) == 0 {
		return
	}
	if q.Multi {
		m.io.Println(i18n.T(i18n.KeyAskUserMultiHint))
	}
	m.io.Println(i18n.T(i18n.KeySettingCmd_601))
	for i, opt := range q.Options {
		m.io.Printf("    [%d] %s\n", i+1, opt)
	}
	for _, k := range keys {
		m.io.Printf("    [%s] %s\n", k.Key, k.Label)
	}
}

// readQuestionAnswer reads the answer of one question, retrying until the input
// is valid. cancelled reports that the user aborted the whole form.
func (m *TerminalInteractionManager) readQuestionAnswer(q Question, keys []KeyOption) (QuestionAnswer, bool, error) {
	ans := QuestionAnswer{Question: q.Title}
	for {
		m.io.Printf(i18n.T(i18n.KeySettingCmd_603))

		input, err := m.io.ReadLine()
		if err != nil {
			return QuestionAnswer{}, false, err
		}

		// A leading space means "free-form note for this question" (FEATURE-438
		// compatible behaviour, kept for the multi-question form).
		if strings.HasPrefix(input, " ") {
			note := strings.TrimSpace(input)
			if note == "" {
				m.io.Println(i18n.T(i18n.KeySettingCmd_604))
				continue
			}
			ans.Selected = nil
			ans.Notes = nil
			ans.Text = note
			return ans, false, nil
		}

		input = strings.TrimSpace(input)
		// Empty input leaves the question unanswered (allowed by design).
		if input == "" {
			return ans, false, nil
		}

		// Fixed keys ("-" aborts the form, others answer the question).
		fixed := false
		for _, k := range keys {
			if input != k.Key {
				continue
			}
			fixed = true
			if k.Value == "think_exit" {
				return QuestionAnswer{}, true, nil
			}
			ans.Selected = nil
			ans.Notes = nil
			ans.Text = k.Label
			return ans, false, nil
		}
		if fixed {
			continue
		}

		// No options: the whole input is the free-form answer.
		if len(q.Options) == 0 {
			ans.Text = input
			return ans, false, nil
		}

		nums, note := splitOptionNumbers(input)
		if len(nums) == 0 {
			// Input does not start with option numbers: treat as free text.
			ans.Selected = nil
			ans.Notes = nil
			ans.Text = input
			return ans, false, nil
		}
		selected, ok := pickOptions(nums, q.Options)
		if !ok {
			m.io.Printf(i18n.T(i18n.KeySettingCmd_608), nums[len(nums)-1])
			continue
		}
		if !q.Multi && len(selected) > 1 {
			m.io.Println(i18n.T(i18n.KeyAskUserSingleHint))
			continue
		}
		ans.Selected = selected
		ans.Notes = nil
		if note != "" {
			if len(selected) == 1 {
				ans.Notes = []AnswerNote{{Option: selected[0], Note: note}}
			} else {
				ans.Text = note
			}
		}
		return ans, false, nil
	}
}

// splitOptionNumbers parses the leading option numbers of an input such as "1",
// "1,3" or "1,3 needs migration", returning the numbers and the remaining text
// as the supplementary note.
func splitOptionNumbers(input string) ([]int, string) {
	var nums []int
	rest := input
	for {
		rest = strings.TrimSpace(rest)
		if rest == "" {
			break
		}
		token := rest
		if i := strings.IndexAny(rest, " \t"); i >= 0 {
			token = rest[:i]
		}
		var local []int
		valid := true
		for _, part := range strings.Split(token, ",") {
			if part == "" {
				continue
			}
			n, err := strconv.Atoi(part)
			if err != nil {
				valid = false
				break
			}
			local = append(local, n)
		}
		if !valid || len(local) == 0 {
			break
		}
		nums = append(nums, local...)
		if len(token) >= len(rest) {
			rest = ""
			break
		}
		rest = rest[len(token):]
	}
	return nums, strings.TrimSpace(rest)
}

// pickOptions maps 1-based option numbers to option texts, rejecting any
// out-of-range number.
func pickOptions(nums []int, options []string) ([]string, bool) {
	selected := make([]string, 0, len(nums))
	for _, n := range nums {
		if n < 1 || n > len(options) {
			return nil, false
		}
		selected = append(selected, options[n-1])
	}
	return selected, true
}

// formatQuestionAnswers renders the collected answers in the compact form
// shared with the web UI (FEATURE-512): one line per question, no repeated
// question text — "Q1: A + C（备注: A→xxx）", "Q2: text", "Q3: （未作答）".
func formatQuestionAnswers(answers []QuestionAnswer) string {
	lines := make([]string, 0, len(answers))
	for i, a := range answers {
		parts := make([]string, 0, 2)
		if len(a.Selected) > 0 {
			parts = append(parts, strings.Join(a.Selected, " + "))
		}
		if a.Text != "" {
			parts = append(parts, a.Text)
		}
		line := fmt.Sprintf("Q%d: %s", i+1, i18n.T(i18n.KeyAskUserNoAnswer))
		if len(parts) > 0 {
			line = fmt.Sprintf("Q%d: %s", i+1, strings.Join(parts, " "))
		}
		for _, n := range a.Notes {
			line += fmt.Sprintf("（%s: %s→%s）", i18n.T(i18n.KeyAskUserNoteLabel), n.Option, n.Note)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
