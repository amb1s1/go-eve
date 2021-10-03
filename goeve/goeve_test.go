package goeve

import (
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	compute "google.golang.org/api/compute/v1"
	"google.golang.org/protobuf/testing/protocmp"
)

var (
	testConfigFile = "../testdata/test_config.yaml"
)

func setup(t *testing.T) (*Client, error) {
	t.Helper()
	c, err := New("", testConfigFile, false)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func TestFirewallRequest(t *testing.T) {
	c, err := setup(t)
	if err != nil {
		t.Errorf("could not create a new goeve client, error: %v", err)
	}

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
				SelfLink:  "projects/testProject/global/firewalls/egress-eve",
				Network:   "projects/testProject/global/networks/default",
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
		got := c.firewallRequest(tc.direction)
		if diff := cmp.Diff(tc.want, got, protocmp.Transform()); diff != "" {
			t.Errorf("ParseAccessString(%s) returned unexpected diff (-want +got):\n%s", tc.direction, diff)
		}

	}
}

func TestInstanceRequest(t *testing.T) {
	c, err := setup(t)
	if err != nil {
		t.Errorf("could not create a new goeve client, error: %v", err)
	}

	tests := []struct {
		name string
		zone string
		want *compute.Instance
	}{
		{
			name: "instance1",
			want: &compute.Instance{
				Name:                   "instance1",
				Description:            "eve-ng compute instance created by go-eve",
				MinCpuPlatform:         "Intel Cascade Lake",
				LastSuspendedTimestamp: "",
				MachineType:            "https://www.googleapis.com/compute/v1/projects/testProject/zones/us-central1-a/machineTypes/c2-standard-4",
				CanIpForward:           true,
				Tags: &compute.Tags{
					Items: []string{
						"http-server",
						"https-server",
						"eve-ng",
					},
				},
				Disks: []*compute.AttachedDisk{
					{
						AutoDelete: true,
						Boot:       true,
						Type:       "PERSISTENT",
						InitializeParams: &compute.AttachedDiskInitializeParams{
							DiskName:    "my-root-instance1",
							SourceImage: "projects/testProject/global/images/test-eve-ng",
							DiskType:    "projects/testProject/zones/us-central1-a/diskTypes/pd-ssd",
						},
					},
				},
				NetworkInterfaces: []*compute.NetworkInterface{
					{
						AccessConfigs: []*compute.AccessConfig{
							{
								Type: "ONE_TO_ONE_NAT",
								Name: "External NAT",
							},
						},
						Network: "https://www.googleapis.com/compute/v1/projects/testProject/global/networks/default",
					},
				},
				ServiceAccounts: []*compute.ServiceAccount{
					{
						Email: "default",
						Scopes: []string{
							compute.DevstorageFullControlScope,
							compute.ComputeScope,
						},
					},
				},
				Metadata: &compute.Metadata{
					Kind:            "compute#metadata",
					ForceSendFields: nil,
					NullFields:      nil,
					Items: []*compute.MetadataItems{
						{
							Key:   "ssh-keys",
							Value: proto.String("eve:ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDIn5Zc9uF4qO8c3e0bxL2jOfPckeuzS56aATA/5aj/Cjx/xiZF+z7t8k5dIg4qX2KJR162iINDnef0XnTPsPs6q6rlVY1ZztZ6OcjqR7bhjfCNVd3s1+zY31uIj3WuorcRzy29yYZUSS7ZTUDXj2ZY5aGDsB47+Cybx/xVsedV83hATB05kQOKFpvRUKdnrnRxjyliwE9C2PbFWViK7sJk9jJ8j69XUONAXobt0IuprgTj6Mvri9uPCq79WDEho4/X8XHChRrNrlwgn5PxqRaYY4eecTNArq50LsknoyNr8S2UbiPdkVe90M1dRXTxdP5Mf/VB3mqSFfnHk9Q9tGMFi2kA4/eCkvMu25FhZ5ReFfgpj2ZScmElqPjxgPZbojmbmZ9zsKYtzmNHdl06taRbj1rEeolpwvRKFaRtcPsA382irX/tk9jrmAIUMcZ4n9/E/mv0refzipXXldwetBIe7t16Kts7aXY6YB1F1qroESlBvBER4xEobOyCUPqMioE= gomdavid@golang\n"),
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		got := c.instanceRequest()
		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Errorf("contructInstanceRequest() returned unexpected diff (-want +got):\n%s", diff)
		}
	}
}

func TestImageRequest(t *testing.T) {
	c, err := setup(t)
	if err != nil {
		t.Errorf("could not create a new goeve client, error: %v", err)
	}

	tests := []struct {
		name string
		want *compute.Image
	}{
		{
			name: "Passed Construction of eve-ng image request",
			want: &compute.Image{
				Name: "test-eve-ng",
				Licenses: []string{
					"https://www.google.com/compute/v1/projects/vm-options/global/licenses/enable-vmx",
				},
				SourceImage: "https://www.googleapis.com/compute/beta/projects/ubuntu-os-cloud/global/images/ubuntu-1604-xenial-v20210429",
				DiskSizeGb:  10,
			},
		},
	}

	for _, tc := range tests {
		got := c.imageRequest()
		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Errorf("contructImageEveImage() returned unexpected diff (-want +got):\n%s", diff)

		}
	}
}
