package attendance

// conflict.go holds the pure decision logic behind the sync algorithm from
// PRD 4.2.5, kept free of database and HTTP concerns specifically so it can be
// unit tested exhaustively -- this is the one piece of the system where a wrong
// answer means silently losing or overwriting a real attendance record, so it
// gets the same treatment as bulkimport/guardianlink: isolated, pure, tested.

// roleRank orders the roles that are ever allowed to write attendance (PRD 2.2:
// only correspondent, office admin and class teacher mark or correct attendance).
// Higher outranks lower (PRD 4.2.5 point 5: "Office admin corrections outrank
// class teacher entries").
func roleRank(role string) int {
	switch role {
	case "teacher":
		return 1
	case "office_admin":
		return 2
	case "correspondent":
		return 3
	default:
		return 0
	}
}

// isReplay reports whether incomingCounter is a stale or duplicate delivery from
// the device that last wrote lastSeenCounter for this entry (PRD 4.2.5 point 2).
// This check is per (entry, device) and has nothing to do with which writer's
// edit should win -- it exists purely so retries and out-of-order arrival from
// the SAME device never apply twice or apply out of order, without trusting
// wall-clock time.
func isReplay(lastSeenCounter, incomingCounter int64) bool {
	return incomingCounter <= lastSeenCounter
}

// resolveConflict decides whether an edit declaring incomingBaseRevision (the
// server_revision the client last saw for this entry, 0 if the client has never
// synced it) should be applied against an entry currently at currentRevision,
// last written by currentRole.
//
// The default when the client's view is stale (incomingBaseRevision <
// currentRevision) is that the server-side change wins (PRD point 4) --
// regardless of arrival order, a client that hasn't seen the latest state
// doesn't get to overwrite it. The one override is authority: an edit from a
// strictly higher-ranked role than whoever made the current version is applied
// anyway (PRD point 5), which is what makes "office admin corrects a student to
// approved sick leave" durable against a class teacher's phone syncing a stale
// offline edit for the same student twenty minutes later.
func resolveConflict(currentRevision int, currentRole string, incomingBaseRevision int, incomingRole string) Outcome {
	if incomingBaseRevision >= currentRevision {
		return OutcomeApplied
	}
	if roleRank(incomingRole) > roleRank(currentRole) {
		return OutcomeApplied
	}
	return OutcomeSuperseded
}
