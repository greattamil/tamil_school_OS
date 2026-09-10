package guardianlink

import "testing"

func TestIsValidIndianMobile(t *testing.T) {
	cases := map[string]bool{
		"9876543210":   true,
		"6000000001":   true,
		"5876543210":   false, // starts with 5, not a valid mobile prefix
		"98765432":     false, // too short
		"987654321012": false, // too long
		"98765abcde":   false,
	}
	for input, want := range cases {
		if got := IsValidIndianMobile(input); got != want {
			t.Errorf("IsValidIndianMobile(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestIsPlaceholder(t *testing.T) {
	cases := map[string]bool{
		"9999999999": true,  // all repeated
		"0000000000": true,  // all repeated
		"1234567890": true,  // ascending
		"9876543210": true,  // descending
		"9840012345": false, // a real-looking number
		"6291234567": false,
	}
	for input, want := range cases {
		if got := IsPlaceholder(input); got != want {
			t.Errorf("IsPlaceholder(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestDecideCluster(t *testing.T) {
	realMobile := "9840012345"

	if d := DecideCluster(realMobile, 1); d != AutoLink {
		t.Errorf("single student on a real number: got %v, want AutoLink", d)
	}
	if d := DecideCluster(realMobile, MaxAutoLinkClusterSize); d != AutoLink {
		t.Errorf("exactly %d students on a real number: got %v, want AutoLink", MaxAutoLinkClusterSize, d)
	}
	if d := DecideCluster(realMobile, MaxAutoLinkClusterSize+1); d != ManualReview {
		t.Errorf("%d students on a real number: got %v, want ManualReview", MaxAutoLinkClusterSize+1, d)
	}

	// A placeholder shared by a driver's dozen kids must still be rejected, never
	// promoted to manual review just because the cluster is large -- there is
	// nothing to adjudicate about a number that was never real.
	if d := DecideCluster("9999999999", 12); d != Rejected {
		t.Errorf("placeholder shared by many rows: got %v, want Rejected", d)
	}
	if d := DecideCluster("9999999999", 1); d != Rejected {
		t.Errorf("placeholder shared by one row: got %v, want Rejected", d)
	}
	if d := DecideCluster("12345", 1); d != Rejected {
		t.Errorf("malformed number: got %v, want Rejected", d)
	}
}

func TestClusterRows(t *testing.T) {
	rows := []string{"9840012345", "9840012345", "", "9999999999", "9840099999"}
	clusters := ClusterRows(rows)

	if got := clusters["9840012345"]; len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Errorf("expected rows 0,1 clustered under 9840012345, got %v", got)
	}
	if _, ok := clusters[""]; ok {
		t.Error("empty mobile should not form a cluster")
	}
	if got := clusters["9999999999"]; len(got) != 1 || got[0] != 3 {
		t.Errorf("expected row 3 clustered under 9999999999, got %v", got)
	}
}
