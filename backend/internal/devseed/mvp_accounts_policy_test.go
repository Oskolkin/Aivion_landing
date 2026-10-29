package devseed

import "testing"

func TestMvpUserPasswordWritePlan(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		resetPassword bool
		wantWrite     bool
		wantReport    bool
	}{
		{"reset false leaves existing hash", false, false, false},
		{"reset true overwrites hash", true, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotW, gotR := mvpUserPasswordWritePlan(tc.resetPassword)
			if gotW != tc.wantWrite || gotR != tc.wantReport {
				t.Fatalf("mvpUserPasswordWritePlan(%v) = (%v, %v), want (%v, %v)",
					tc.resetPassword, gotW, gotR, tc.wantWrite, tc.wantReport)
			}
		})
	}
}
