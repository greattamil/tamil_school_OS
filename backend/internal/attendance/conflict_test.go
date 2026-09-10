package attendance

import "testing"

func TestIsReplay(t *testing.T) {
	cases := []struct {
		name       string
		lastSeen   int64
		incoming   int64
		wantReplay bool
	}{
		{"first ever write from device", 0, 1, false},
		{"strictly increasing counter", 5, 6, false},
		{"exact duplicate delivery", 5, 5, true},
		{"stale out-of-order delivery", 5, 3, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isReplay(c.lastSeen, c.incoming); got != c.wantReplay {
				t.Errorf("isReplay(%d, %d) = %v, want %v", c.lastSeen, c.incoming, got, c.wantReplay)
			}
		})
	}
}

func TestResolveConflict_NoConflict(t *testing.T) {
	// Client's base revision matches (or exceeds, defensively) the current
	// server revision: nothing else has changed underneath it, apply cleanly.
	if got := resolveConflict(1, "teacher", 1, "teacher"); got != OutcomeApplied {
		t.Errorf("matching base revision: got %v, want Applied", got)
	}
	if got := resolveConflict(0, "", 0, "teacher"); got != OutcomeApplied {
		t.Errorf("brand-new entry (revision 0): got %v, want Applied", got)
	}
}

func TestResolveConflict_OfficeAdminCorrectionSurvivesStaleTeacherSync(t *testing.T) {
	// The exact scenario from PRD 4.2.5: a class teacher marks 6A at 08:45
	// offline. At 09:00 the office admin records approved sick leave through the
	// web panel (this is what creates revision 1, authored by office_admin).
	// At 09:15 the teacher's phone finally syncs its offline edit, which was
	// based on revision 0 (the teacher never saw the office admin's change).
	// The teacher's edit must be rejected -- the office admin's correction wins.
	got := resolveConflict(1 /* currentRevision */, "office_admin", 0 /* incomingBaseRevision */, "teacher")
	if got != OutcomeSuperseded {
		t.Fatalf("stale teacher sync against an office admin correction: got %v, want Superseded", got)
	}
}

func TestResolveConflict_OfficeAdminOverridesEvenWhenItLosesTheRace(t *testing.T) {
	// A teacher's edit is somehow processed first (revision becomes 1, authored
	// by teacher). An office admin correction arrives declaring a stale base
	// revision (0) -- e.g. the admin started the correction before seeing the
	// teacher's entry. Role authority overrides the race outcome: the
	// office admin's edit still applies (PRD point 5), advancing the revision,
	// and a subsequent stale sync from anyone lower-ranked would then be
	// rejected against it.
	got := resolveConflict(1 /* currentRevision, authored by teacher */, "teacher", 0, "office_admin")
	if got != OutcomeApplied {
		t.Fatalf("office admin correction racing a teacher entry: got %v, want Applied", got)
	}
}

func TestResolveConflict_TeacherCannotOverrideStaleCorrespondentEdit(t *testing.T) {
	got := resolveConflict(2, "correspondent", 1, "teacher")
	if got != OutcomeSuperseded {
		t.Fatalf("teacher edit against a correspondent's newer edit: got %v, want Superseded", got)
	}
}

func TestResolveConflict_EqualAuthorityStaleEditIsSuperseded(t *testing.T) {
	// Two teachers (e.g. a class teacher and an unexpected second writer) at the
	// same rank: whoever's change is already on the server wins against a
	// stale-based edit from an equally-ranked writer. There is no tie-break in
	// the writer's favour just because ranks match.
	got := resolveConflict(2, "teacher", 1, "teacher")
	if got != OutcomeSuperseded {
		t.Fatalf("equal-rank stale edit: got %v, want Superseded", got)
	}
}

func TestRoleRank_Ordering(t *testing.T) {
	if roleRank("correspondent") <= roleRank("office_admin") {
		t.Error("correspondent must outrank office_admin")
	}
	if roleRank("office_admin") <= roleRank("teacher") {
		t.Error("office_admin must outrank teacher")
	}
	if roleRank("teacher") <= roleRank("") {
		t.Error("teacher must outrank an unrecognised/empty role")
	}
	if roleRank("parent") != 0 {
		t.Error("parent never writes attendance and should rank at the floor (0), same as unrecognised")
	}
}
