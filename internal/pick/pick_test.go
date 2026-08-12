// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

package pick

import (
	"testing"

	"github.com/MorganKryze/tunny/internal/config"
)

func demo() Picker {
	return Picker{All: []config.Destination{
		{Name: "hyperviseur", Desc: "Cockpit"},
		{Name: "gateway", Desc: "Duplicati"},
		{Name: "vm-backup", Desc: "Duplicati d'une VM"},
	}}
}

func TestCursorStopsAtBothEnds(t *testing.T) {
	p := demo()
	// Up at the top stays at the top rather than wrapping: wrapping in a
	// three-line list reads as a glitch, not as a feature.
	if next, _, _ := p.Update(KeyUp, 0); next.Cursor != 0 {
		t.Fatalf("up at the top has to stay at 0, got %d", next.Cursor)
	}
	for range 5 {
		p, _, _ = p.Update(KeyDown, 0)
	}
	if p.Cursor != 2 {
		t.Fatalf("down at the end has to stay at 2, got %d", p.Cursor)
	}
}

func TestFilterNarrowsAndKeepsTheCursorValid(t *testing.T) {
	p := demo()
	p.Cursor = 2 // on vm-backup
	for _, r := range "hyper" {
		p, _, _ = p.Update(KeyRune, r)
	}
	if got := p.Matches(); len(got) != 1 || got[0].Name != "hyperviseur" {
		t.Fatalf("the filter has to keep the one match, got %v", got)
	}
	// The cursor pointed at the third row, and there is only one row left.
	if p.Cursor != 0 {
		t.Fatalf("the cursor has to stay inside the filtered list, got %d", p.Cursor)
	}
}

// A filter matching nothing must not make Enter select a row that is not
// there. This is the crash the test exists to prevent.
func TestAFilterMatchingNothingSelectsNothing(t *testing.T) {
	p := demo()
	for _, r := range "zzz" {
		p, _, _ = p.Update(KeyRune, r)
	}
	if len(p.Matches()) != 0 {
		t.Fatal("the filter should have left nothing")
	}
	next, chosen, done := p.Update(KeyEnter, 0)
	if done || chosen != "" {
		t.Fatalf("enter on an empty list must choose nothing, got %q done=%v", chosen, done)
	}
	if next.Cursor != 0 {
		t.Fatalf("invalid cursor: %d", next.Cursor)
	}
	// Down on an empty list must not walk the cursor off the end either.
	if next, _, _ = next.Update(KeyDown, 0); next.Cursor != 0 {
		t.Fatalf("down on an empty list has to stay at 0, got %d", next.Cursor)
	}
}

func TestBackspaceWidensAgain(t *testing.T) {
	p := demo()
	for _, r := range "hyper" {
		p, _, _ = p.Update(KeyRune, r)
	}
	for range 5 {
		p, _, _ = p.Update(KeyBackspace, 0)
	}
	if len(p.Matches()) != 3 {
		t.Fatalf("an emptied filter has to show everything again, got %d", len(p.Matches()))
	}
	// Backspace on an empty filter is a no-op, not a panic.
	if next, _, _ := p.Update(KeyBackspace, 0); next.Filter != "" {
		t.Fatalf("want an empty filter, got %q", next.Filter)
	}
}

func TestEnterAndEscape(t *testing.T) {
	p := demo()
	p, _, _ = p.Update(KeyDown, 0)
	_, chosen, done := p.Update(KeyEnter, 0)
	if !done || chosen != "gateway" {
		t.Fatalf("enter has to choose the row under the cursor, got %q done=%v", chosen, done)
	}

	_, chosen, done = demo().Update(KeyEsc, 0)
	if !done || chosen != "" {
		t.Fatalf("escape has to finish without choosing, got %q done=%v", chosen, done)
	}
}

func TestFilterMatchesTheDescriptionToo(t *testing.T) {
	p := demo()
	for _, r := range "duplicati" {
		p, _, _ = p.Update(KeyRune, r)
	}
	if got := p.Matches(); len(got) != 2 {
		t.Fatalf("the filter has to read desc too, got %d matches", len(got))
	}
}

// A filtered Enter must select what is under the cursor in the *filtered*
// list, not the row that sat at that index before typing.
func TestEnterSelectsFromTheFilteredList(t *testing.T) {
	p := demo()
	for _, r := range "duplicati" {
		p, _, _ = p.Update(KeyRune, r)
	}
	p, _, _ = p.Update(KeyDown, 0)
	if _, chosen, _ := p.Update(KeyEnter, 0); chosen != "vm-backup" {
		t.Fatalf("want vm-backup, got %q", chosen)
	}
}

// An unmapped key must leave the state exactly as it was: no filter change,
// no cursor move, no selection.
func TestAnUnknownKeyChangesNothing(t *testing.T) {
	p := demo()
	p, _, _ = p.Update(KeyDown, 0)
	next, chosen, done := p.Update(KeyNone, 0)
	if done || chosen != "" || next.Cursor != 1 || next.Filter != "" {
		t.Fatalf("KeyNone has to be neutral, got %+v chosen=%q done=%v", next, chosen, done)
	}
}
