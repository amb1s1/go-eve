// Copyright 2017 Google LLC.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goeve

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"strings"
	"time"

	"github.com/amb1s1/go-eve/connect"

	evecompute "github.com/amb1s1/go-eve/eve-compute"

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
		SourceImage: "https://www.googleapis.com/compute/beta/projects/ubuntu-os-cloud/global/images/ubuntu-1604-xenial-v20210429",
		DiskSizeGb:  10,
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

func (c *Client) InitialSetup(publicKey, privateKey, username, instanceStatus string, ip net.Addr) error {
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
		imageCreated := service.IsImageCreated(c.ProjectID, c.CustomEveNGImageName)
		if !imageCreated {
			err := service.CreateImage(c.ProjectID, eveImage)
			if err != nil {
				return err
			}
			return nil
		}
		log.Printf("Custom Image name: %v is already created. Skipping new custom image creation.", c.CustomEveNGImageName)
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

func (c *Client) setupInstance(service evecompute.ServiceFunctions, instanceStatus string) error {
	ip, err := service.GetExternalIP(c.ProjectID, c.Zone, c.InstanceName)
	if err != nil {
		return err
	}
	err = c.InitialSetup(c.SSHPublicKeyFileName, c.SSHPrivateKeyFileName, c.SSHKeyUsername, instanceStatus, ip)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) teardown(service evecompute.ServiceFunctions) error {
	err := service.DeleteInstance(c.ProjectID, c.Zone, c.InstanceName)
	if err != nil {
		return err
	}
	c.Status.Instance = "Deleted"
	c.Status.Settings = "Gone with the Instance"

	err = service.DeleteFirewallRules(c.ProjectID)
	if err != nil {
		return err
	}
	c.Status.Firewall.Egress = "Deleted"
	c.Status.Firewall.Ingress = "Deleted"

	err = service.DeleteImage(c.ProjectID, c.CustomEveNGImageName)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) create(instanceStatus string, resetInstance bool, service evecompute.ServiceFunctions) error {
	if instanceStatus == "TERMINATED" {
		log.Printf("instance %v is not running, starting instance", c.InstanceName)
		err := service.StartInstance(c.ProjectID, c.Zone, c.InstanceName)
		if err != nil {
			return nil
		}
		instanceStatus = "RUNNING"
	}

	if instanceStatus == "RUNNING" && !resetInstance {
		log.Printf("instance %v already exists", c.InstanceName)
		c.Status.Instance = "instance " + c.InstanceName + " not modified"
	}

	if instanceStatus == "RUNNING" && resetInstance {
		err := service.DeleteInstance(c.ProjectID, c.Zone, c.InstanceName)
		if err != nil {
			log.Fatalf("could not delete instance %v, error: %v", c.InstanceName, err)
		}
		instanceStatus = ""
	}

	if instanceStatus == "" {
		if err := c.createInstance(service); err != nil {
			log.Printf("createInstance() could not create a new instance, error: %v", err)
		}
		c.Status.Instance = "new instance " + c.InstanceName + " was created"
	}
	if c.Status.Firewall.Ingress == "" {
		c.Status.Firewall.Ingress = "created"
	}
	if c.Status.Firewall.Egress == "" {
		c.Status.Firewall.Egress = "created"
	}

	if err := c.setupInstance(service, instanceStatus); err != nil {
		log.Fatalf("could not setup the new instance, error: %v", err)
	}
	return nil
}

func (c *Client) stop(instanceStatus string, service evecompute.ServiceFunctions) error {
	if instanceStatus == "" {
		log.Fatalf("Could not stop instance %v, instance does not exists", c.InstanceName)
	}
	if instanceStatus == "TERMINATED" {
		log.Fatalf("Could not stop instance %v, instance is not running", c.InstanceName)
	}
	err := service.StopInstance(c.ProjectID, c.Zone, c.InstanceName)
	if err != nil {
		return nil
	}

	c.Status.Instance = "Stopped"
	c.Status.Firewall.Egress = "Not modified"
	c.Status.Firewall.Ingress = "Not modified"
	c.Status.Settings = "Not modified"
	return nil
}

func Run(instanceName, configFile string, createCustomEveNGImage, createLab, resetInstance, stop, teardown bool) *status {
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

	instanceStatus := service.InstanceStatus(c.ProjectID, c.Zone, c.InstanceName)

	switch {
	case teardown:
		if err := c.teardown(service); err != nil {
			log.Fatalf("could not teardown lab for compute instance %v, error: %v", c.InstanceName, err)
		}
	case stop:
		if err := c.stop(instanceStatus, service); err != nil {
			fmt.Printf("Could not stop compute instance %v, error: %v", c.InstanceName, err)
		}
	case createLab:
		if err := c.create(instanceStatus, resetInstance, service); err != nil {
			fmt.Printf("Could not ")
		}

	}

	return c.Status

}
