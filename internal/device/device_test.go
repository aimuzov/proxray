package device

import "testing"

func TestHWIDFromIsStableAndAcceptable(t *testing.T) {
	const machine = "9C2F1B4E-0000-4A11-9E7C-1D2E3F405162"

	id := hwidFrom(machine)
	if id != hwidFrom(machine) {
		t.Error("hwidFrom is not deterministic")
	}
	if !Valid(id) {
		t.Errorf("hwidFrom produced %q, which panels reject", id)
	}
	// The point of hashing is that the platform UUID stays on the machine.
	if id == machine {
		t.Error("hwidFrom leaked the raw machine id")
	}
	if other := hwidFrom(machine + "x"); other == id {
		t.Error("two machine ids collided")
	}
}

func TestRandomIsAcceptableAndFresh(t *testing.T) {
	first, err := Random()
	if err != nil {
		t.Fatalf("Random: %v", err)
	}
	if !Valid(first) {
		t.Errorf("Random produced %q, which panels reject", first)
	}
	second, err := Random()
	if err != nil {
		t.Fatalf("Random: %v", err)
	}
	if first == second {
		t.Error("Random repeated itself")
	}
}

func TestValid(t *testing.T) {
	tests := map[string]bool{
		"abcdef0123":     true,  // shortest the panel accepts
		"AZaz09=-":       false, // too short
		"has space":      false,
		"underscore_no":  false,
		"плохой-юникод":  false,
		"a1b2c3d4e5f6=-": true,
	}
	for id, want := range tests {
		if got := Valid(id); got != want {
			t.Errorf("Valid(%q) = %v, want %v", id, got, want)
		}
	}
}

// Detect runs against whatever machine the tests run on, so it can only be held
// to the contract: a usable HWID or none at all.
func TestDetect(t *testing.T) {
	info := Detect()
	if info.HWID != "" && !Valid(info.HWID) {
		t.Errorf("Detect returned HWID %q, which panels reject", info.HWID)
	}
	if info.OS == "" {
		t.Error("Detect returned no OS name")
	}
}
