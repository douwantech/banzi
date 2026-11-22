package main

import "testing"

// Keeps the manual whole: every command has a name, group, index line, help and a run.
func TestCommandsComplete(t *testing.T) {
	registerAll()
	if len(commands) == 0 {
		t.Fatal("no commands registered")
	}
	seen := map[string]bool{}
	for _, c := range commands {
		if c.name == "" || c.group == "" || c.one == "" || c.help == "" {
			t.Errorf("command %q missing name/group/one/help", c.name)
		}
		if c.run == nil {
			t.Errorf("command %q has no run", c.name)
		}
		if seen[c.name] {
			t.Errorf("duplicate command %q", c.name)
		}
		seen[c.name] = true
	}
}
