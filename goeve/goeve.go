// Copyright 2017 Google LLC.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goeve

import (
	"go-eve/connect"
	evecompute "go-eve/eve-compute"
	"io/ioutil"
	"log"
	"net"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"gopkg.in/yaml.v2"

	compute "google.golang.org/api/compute/v1"
)

var (
	bashFiles  = []string{"install.sh", "eve-initial-setup.sh"}
	configFile = "config.yaml"
)

type Client struct {
	ProjectID              string `yaml:"projectID"`
	InstanceName           string `yaml:"instanceName"`
	Zone                   string `yaml:"zone"`
	SSHPublicKeyFileName   string `yaml:"sshPublicKeyFileName"`
	SSHPrivateKeyFileName  string `yaml:"sshPrivateKeyFileName"`
	SSHKeyUsername         string `yaml:"sshKeyUsername"`
	CustomEveNGImageName   string `yaml:"customEveNGImageName"`
	createCustomEveNGImage bool
}

func NewClient(instanceName, oConfigFile string, createCustomEveNGImage bool) (*Client, error) {
	c := &Client{}
	if oConfigFile != "" {
		configFile = oConfigFile
	}
	file, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatalf("could not read public ssh file error: %v", err)
	}

	if err = yaml.Unmarshal(file, &c); err != nil {
		return nil, err
	}
	if instanceName != "" {
		c.InstanceName = instanceName
	}
	c.createCustomEveNGImage = createCustomEveNGImage

	return c, nil
}

func (c *Client) constructFirewallRules(direction string) *compute.Firewall {
	rule := &compute.Firewall{
		Kind:      "compute#firewall",
		Name:      strings.ToLower(direction) + "-eve",
		SelfLink:  "projects/" + c.ProjectID + "/global/firewalls/" + strings.ToLower(direction) + "-eve",
		Network:   "projects/" + c.ProjectID + "/global/networks/default",
		Direction: strings.ToUpper(direction),
		Priority:  1000,
		TargetTags: []string{
			"eve-ng",
		},
		Allowed: []*compute.FirewallAllowed{
			{
				IPProtocol: "tcp",
				Ports: []string{
					"0-65535",
				},
			},
		},
	}

	if strings.ToUpper(direction) != "INGRESS" {
		rule.DestinationRanges = append(rule.DestinationRanges, "0.0.0.0/0")
		return rule
	}
	rule.SourceRanges = append(rule.SourceRanges, "0.0.0.0/0")

	return rule
}

func (c *Client) contructInstanceRequest() *compute.Instance {
	prefix := "https://www.googleapis.com/compute/v1/projects/" + c.ProjectID
	sshKey := c.readSSHKey()

	instance := &compute.Instance{
		Name:           c.InstanceName,
		Description:    "compute sample instance",
		MinCpuPlatform: "Intel Cascade Lake",
		MachineType:    prefix + "/zones/" + c.Zone + "/machineTypes/c2-standard-4",
		CanIpForward:   true,
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
					DiskName:    "my-root-" + c.InstanceName,
					SourceImage: "projects/" + c.ProjectID + "/global/images/" + c.CustomEveNGImageName,
					DiskType:    "projects/" + c.ProjectID + "/zones/us-central1-a/diskTypes/pd-ssd",
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
				Network: prefix + "/global/networks/default",
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
			Kind: "compute#metadata",
			Items: []*compute.MetadataItems{
				{
					Key:   "ssh-keys",
					Value: proto.String(c.SSHKeyUsername + ":" + string(sshKey)),
				},
			},
		},
	}
	return instance

}

func (c *Client) ConstructEveImage() *compute.Image {
	image := &compute.Image{
		Name: c.CustomEveNGImageName,
		Licenses: []string{
			"https://www.google.com/compute/v1/projects/vm-options/global/licenses/enable-vmx",
		},
		SourceDisk: "https://www.googleapis.com/compute/beta/projects/ubuntu-os-cloud/global/images/ubuntu-1604-xenial-v20210429",
		DiskSizeGb: 10,
	}
	return image
}

func (c *Client) readSSHKey() []byte {
	body, err := ioutil.ReadFile(c.SSHPublicKeyFileName)
	if err != nil {
		log.Fatalf("could not read public ssh file %v, error: %v", c.SSHPublicKeyFileName, err)
	}
	return body
}

func (c *Client) InitialSetup(publicKey, privateKey, username string, ip net.Addr) error {
	for _, f := range bashFiles {
		client, err := connect.NewClient(publicKey, privateKey, username, ip)
		if err != nil {
			return err
		}
		err = client.Fetch(f)
		if err != nil {
			log.Println(err)
		}

		err = client.RunScript(f)
		if err != nil {
			return err
		}

		client.Reboot()
		time.Sleep(60 * time.Second)
	}
	return nil
}

func Run(instanceName, configFile string, createCustomEveNGImage bool) {
	client, err := NewClient(instanceName, configFile, createCustomEveNGImage)
	if err != nil {
		log.Fatalf("could not create a new goeve client, error: %v", err)
	}

	eveImage := client.ConstructEveImage()
	instance := client.contructInstanceRequest()
	iFirewall := client.constructFirewallRules("INGRESS")
	eFirewall := client.constructFirewallRules("EGRESS")
	service, err := evecompute.NewClient()
	if err != nil {
		log.Fatalf("Could not create a new google compute service, error: %v", err)
	}
	if client.createCustomEveNGImage {
		err = service.CreateImage(client.ProjectID, eveImage)
		if err != nil {
			log.Fatalf("error creating a new eve ng image, error: %v", err)
		}

	}

	if service.IsInstanceExists(client.ProjectID, client.Zone, client.InstanceName) {
		log.Fatalf("instance %v already exists", client.InstanceName)
	}

	err = service.CreateInstance(client.ProjectID, client.Zone, instance)
	if err != nil {
		log.Fatalf("could not create a new compute instance, error: %v", err)
	}

	err = service.InsertFireWallRule(client.ProjectID, iFirewall)
	if err != nil {
		log.Printf("could not insert the ingress rule, error: %v", err)
	}

	err = service.InsertFireWallRule(client.ProjectID, eFirewall)
	if err != nil {
		log.Printf("could not insert the egress rule, error: %v", err)
	}

	ip, err := service.GetExternalIP(client.ProjectID, client.Zone, client.InstanceName)
	if err != nil {
		log.Fatalf("Could not get compute instance external ip, error: %v", err)
	}
	err = client.InitialSetup(client.SSHPublicKeyFileName, client.SSHPrivateKeyFileName, client.SSHKeyUsername, ip)
	if err != nil {
		log.Println(err)
	}

}
