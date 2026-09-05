package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestColorModes(t *testing.T) {
	u, err := newUI("never", "never")
	if err != nil {
		t.Fatal(err)
	}
	if got := u.renderStyle("good", "PASS"); strings.Contains(got, "\x1b[") {
		t.Fatalf("never mode emitted ANSI: %q", got)
	}

	u, err = newUI("always", "never")
	if err != nil {
		t.Fatal(err)
	}
	if got := u.renderStyle("good", "PASS"); !strings.Contains(got, "\x1b[") {
		t.Fatalf("always mode did not emit ANSI: %q", got)
	}

	if _, err := newUI("banana", "never"); err == nil {
		t.Fatal("expected invalid color mode error")
	}
}

func TestRichTableAlignmentIgnoresANSI(t *testing.T) {
	u, _ := newUI("always", "never")
	out := u.table(
		[]string{"NAME", "STATE"},
		[][]tableCell{
			{accentCell("a"), goodCell("PASS")},
			{accentCell("longer"), badCell("FAIL")},
		},
	)
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "╭") || !strings.Contains(plain, "╯") {
		t.Fatalf("expected rounded table frame: %q", plain)
	}
	if !strings.Contains(plain, "│ a      │ PASS  │") {
		t.Fatalf("first row is not aligned: %q", plain)
	}
	if !strings.Contains(plain, "│ longer │ FAIL  │") {
		t.Fatalf("second row is not aligned: %q", plain)
	}
}

func TestBadgeAndPanelStructure(t *testing.T) {
	u, _ := newUI("never", "never")
	if got := u.badge("good", "pass"); !strings.Contains(got, "PASS") {
		t.Fatalf("badge missing text: %q", got)
	}
	panel := u.panel("Ready", "Everything works", "good")
	if !strings.Contains(panel, "╭") || !strings.Contains(panel, "Ready") || !strings.Contains(panel, "Everything works") {
		t.Fatalf("unexpected panel: %q", panel)
	}
}

func TestHyperlinkModes(t *testing.T) {
	u, err := newUI("never", "always")
	if err != nil {
		t.Fatal(err)
	}
	got := u.link("repo", "https://github.com/example/repo")
	if !strings.Contains(got, "\x1b]8;;https://github.com/example/repo\x1b\\") {
		t.Fatalf("always mode missing OSC 8 hyperlink: %q", got)
	}

	u, err = newUI("never", "never")
	if err != nil {
		t.Fatal(err)
	}
	if got := u.link("repo", "https://github.com/example/repo"); got != "repo" {
		t.Fatalf("never mode emitted hyperlink: %q", got)
	}
}

func TestLinkedTableAlignmentUsesPlainCellWidth(t *testing.T) {
	u, _ := newUI("always", "always")
	out := u.table(
		[]string{"REPOSITORY", "STATE"},
		[][]tableCell{
			{linkedAccentCell("a/b", "https://github.com/a/b"), goodCell("RUNNING")},
			{linkedAccentCell("long/repo", "https://github.com/long/repo"), mutedCell("IDLE")},
		},
	)
	plain := ansi.Strip(out)
	// OSC 8 is not a CSI sequence, so strip it explicitly for structural assertions.
	plain = strings.ReplaceAll(plain, "\x1b]8;;https://github.com/a/b\x1b\\", "")
	plain = strings.ReplaceAll(plain, "\x1b]8;;https://github.com/long/repo\x1b\\", "")
	plain = strings.ReplaceAll(plain, "\x1b]8;;\x1b\\", "")
	if !strings.Contains(plain, "│ a/b        │ RUNNING │") {
		t.Fatalf("linked first row is not aligned: %q", plain)
	}
	if !strings.Contains(plain, "│ long/repo  │ IDLE    │") {
		t.Fatalf("linked second row is not aligned: %q", plain)
	}
}
