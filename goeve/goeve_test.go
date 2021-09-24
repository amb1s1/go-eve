package goeve

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	compute "google.golang.org/api/compute/v1"
	"google.golang.org/protobuf/testing/protocmp"
)

func setup(t *testing.T) *Client {
	t.Helper()
	client := NewClient("testProject", "instance1", "us-central1-a", "testdata/rsa.pub", "testdata/rsa", "eveuser", "test-eve-ng-image", false)
	return client
}
func TestConstructFilewallRules(t *testing.T) {
	client := setup(t)
	tests := []struct {
		name      string
		direction string
		want      *compute.Firewall
	}{
		{
			name:      "Passing egress firerule",
			direction: "EGRESS",
			want: &compute.Firewall{
				Kind:      "compute#firewall",
				Name:      "egress-eve",
				SelfLink:  "projects/amb1s1/global/firewalls/egress-eve",
				Network:   "projects/amb1s1/global/networks/default",
				Direction: "EGRESS",
				Priority:  1000,
				TargetTags: []string{
					"eve-ng",
				},
				DestinationRanges: []string{
					"0.0.0.0/0",
				},
				Allowed: []*compute.FirewallAllowed{
					{
						IPProtocol: "tcp",
						Ports: []string{
							"0-65535",
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		got := client.cronstructFirewallRules(tc.direction)
		if diff := cmp.Diff(tc.want, got, protocmp.Transform()); diff != "" {
			t.Errorf("ParseAccessString(%s) returned unexpected diff (-want +got):\n%s", tc.direction, diff)
		}

	}
}
