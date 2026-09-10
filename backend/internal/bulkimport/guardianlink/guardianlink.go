// Package guardianlink implements the safety heuristics required before a bulk
// import may automatically link students to a guardian by shared mobile number
// (PRD 4.1.3). Linking siblings by matching mobile numbers is correct in principle
// and dangerous in practice: school spreadsheets routinely contain a van driver's
// number pasted down a column, the office's own number used as a placeholder, or
// a family's shared number covering children who are not siblings for fee
// purposes. Auto-linking on dirty data silently hands one guardian account access
// to unrelated children's records -- a data breach produced by a convenience
// feature. These functions decide, per mobile-number cluster, whether that is safe
// to do automatically, must be rejected outright, or needs a human to look at it.
package guardianlink

import "regexp"

// Decision is the outcome for one mobile-number cluster (every import row sharing
// that mobile number).
type Decision string

const (
	// AutoLink: safe to create one guardian and link every row in the cluster to
	// it without a human in the loop.
	AutoLink Decision = "auto_link"
	// ManualReview: more than four students share this number. Surname/address
	// divergence raises suspicion further but does not by itself clear a cluster
	// back to AutoLink -- a human decides either way.
	ManualReview Decision = "manual_review"
	// Rejected: the number is a known placeholder or fails basic Indian mobile
	// validation. The affected rows import with no guardian contact rather than
	// a false one.
	Rejected Decision = "rejected"
)

// MaxAutoLinkClusterSize is the PRD 4.1.3 threshold: "a number linking more than
// four students blocks automatic merging."
const MaxAutoLinkClusterSize = 4

var indianMobilePattern = regexp.MustCompile(`^[6-9]\d{9}$`)

// IsValidIndianMobile reports whether mobile is a plausible 10-digit Indian mobile
// number (starts 6-9, matching TRAI's numbering plan for mobile subscribers).
func IsValidIndianMobile(mobile string) bool {
	return indianMobilePattern.MatchString(mobile)
}

// IsPlaceholder reports whether mobile matches a known placeholder pattern:
// all-repeated digits (9999999999), or a monotonic digit sequence (1234567890,
// 9876543210, wrapping 9->0 and 0->9) -- the patterns spreadsheets fill in when a
// real number was never collected.
func IsPlaceholder(mobile string) bool {
	if len(mobile) != 10 {
		return false
	}

	allSame := true
	ascending := true
	descending := true
	for i := 1; i < len(mobile); i++ {
		prev := int(mobile[i-1] - '0')
		cur := int(mobile[i] - '0')
		if mobile[i] != mobile[0] {
			allSame = false
		}
		if cur != (prev+1)%10 {
			ascending = false
		}
		if cur != (prev+9)%10 { // (prev - 1) mod 10, avoiding negative modulo
			descending = false
		}
	}
	return allSame || ascending || descending
}

// DecideCluster returns the decision for a mobile number shared by clusterSize
// import rows. Rejection is checked before size: a placeholder shared by two rows
// is still rejected, not auto-linked, and a placeholder shared by fifty rows is
// still reported as "rejected" rather than "needs review" -- there is nothing for
// a human to adjudicate about a number that was never real.
func DecideCluster(mobile string, clusterSize int) Decision {
	if !IsValidIndianMobile(mobile) || IsPlaceholder(mobile) {
		return Rejected
	}
	if clusterSize > MaxAutoLinkClusterSize {
		return ManualReview
	}
	return AutoLink
}

// ClusterRows groups row indices by mobile number, in first-seen order, so callers
// can report clusters deterministically.
func ClusterRows(mobilesByRow []string) map[string][]int {
	clusters := make(map[string][]int)
	for i, mobile := range mobilesByRow {
		if mobile == "" {
			continue
		}
		clusters[mobile] = append(clusters[mobile], i)
	}
	return clusters
}
