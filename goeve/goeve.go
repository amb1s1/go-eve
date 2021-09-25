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

type Firewalls struct {
	Ingress string
	Egress  string
}
type status struct {
	Instance string
	Settings string
	Firewall Firewalls
}

type Client struct {
	ProjectID              string `yaml:"projectID"`
	InstanceName           string `yaml:"instanceName"`
	Zone                   string `yaml:"zone"`
	SSHPublicKeyFileName   string `yaml:"sshPublicKeyFileName"`
	SSHPrivateKeyFileName  string `yaml:"sshPrivateKeyFileName"`
	SSHKeyUsername         string `yaml:"sshKeyUsername"`
	CustomEveNGImageName   string `yaml:"customEveNGImageName"`
	createCustomEveNGImage bool
	Status                 *status
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

func (c *Client) InitialSetup(publicKey, privateKey, username string, isInstanceExists bool, ip net.Addr) error {
	for _, f := range bashFiles {
		client, err := connect.NewClient(publicKey, privateKey, username, ip)
		if err != nil {
			return err
		}
		err = client.Fetch(f)
		if err != nil {
			log.Println(err)
		}

		out, err := client.RunScript(f)
		if err != nil {
			return err
		}
		if string(out) == "VM is alredy configured\n" {
			log.Println(string(out))
			c.Status.Settings = "not modified"
			break
		}
		client.Reboot()
		time.Sleep(60 * time.Second)

	}
	c.Status.Settings = "configured"
	return nil
}

func (c *Client) createInstance(service evecompute.ServiceFunctions) error {
	eveImage := c.ConstructEveImage()
	instance := c.contructInstanceRequest()
	iFirewall := c.constructFirewallRules("INGRESS")
	eFirewall := c.constructFirewallRules("EGRESS")
	if c.createCustomEveNGImage {
		err := service.CreateImage(c.ProjectID, eveImage)
		if err != nil {
			return err
		}
	}

	err := service.CreateInstance(c.ProjectID, c.Zone, instance)
	if err != nil {
		return err
	}

	err = service.InsertFireWallRule(c.ProjectID, iFirewall)
	if err != nil {
		c.Status.Firewall.Ingress = "not modified"
		log.Println(err)
	}

	err = service.InsertFireWallRule(c.ProjectID, eFirewall)
	if err != nil {
		c.Status.Firewall.Egress = "not modified"
		log.Println(err)
	}
	return nil

}

func (c *Client) setupInstance(service evecompute.ServiceFunctions, isInstanceExists bool) error {
	ip, err := service.GetExternalIP(c.ProjectID, c.Zone, c.InstanceName)
	if err != nil {
		return err
	}
	err = c.InitialSetup(c.SSHPublicKeyFileName, c.SSHPrivateKeyFileName, c.SSHKeyUsername, isInstanceExists, ip)
	if err != nil {
		return err
	}
	return nil
}

func Run(instanceName, configFile string, createCustomEveNGImage, resetInstance bool) *status {
	c, err := NewClient(instanceName, configFile, createCustomEveNGImage)
	c.Status = &status{
		Firewall: Firewalls{},
	}
	if err != nil {
		log.Fatalf("could not create a new goeve client, error: %v", err)
	}

	service, err := evecompute.NewClient()
	if err != nil {
		log.Fatalf("Could not create a new google compute service, error: %v", err)
	}

	isInstanceExists := service.IsInstanceExists(c.ProjectID, c.Zone, instanceName)

	if isInstanceExists && !resetInstance {
		log.Printf("instance %v already exists", c.InstanceName)
		c.Status.Instance = "instance " + instanceName + " not modified"
	}

	if isInstanceExists && resetInstance {
		err := service.DeleteInstance(c.ProjectID, c.Zone, instanceName)
		if err != nil {
			log.Fatalf("could not delete instance %v, error: %v", instanceName, err)
		}
		isInstanceExists = false
	}

	if !isInstanceExists {
		if err := c.createInstance(service); err != nil {
			log.Fatalf("could not create a new instance, error: %v", err)
		}
		c.Status.Instance = "new instance " + instanceName + " was created"
	}
	log.Printf("debug firewall, %v", c.Status.Firewall.Egress)
	if c.Status.Firewall.Ingress == "" {
		c.Status.Firewall.Ingress = "created"
	}
	if c.Status.Firewall.Egress == "" {
		c.Status.Firewall.Egress = "created"
	}

	if err := c.setupInstance(service, isInstanceExists); err != nil {
		log.Fatalf("could not setup the new instance, error: %v", err)
	}
	return c.Status
}
