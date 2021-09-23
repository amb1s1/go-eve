// Copyright 2017 Google LLC.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goeve

import (
	"context"
	"errors"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/melbahja/goph"
	"golang.org/x/oauth2/google"

	compute "google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
)

var (
	bashFiles = []string{"install.sh", "eve-initial-setup.sh"}
)

type Client struct {
	projectID             string
	instanceName          string
	zone                  string
	sshPublicKeyFileName  string
	sshPrivateKeyFileName string
	sshKeyUsername        string
	customEveNGImageName  string
	service               *compute.Service
}

func NewClient(ctx context.Context, projectID, instanceName, zone, sshPublicKeyFileName, sshPrivateKeyFileName, sshKeyUsername, customEveNGImageName string) (*Client, error) {
	client := &Client{
		projectID:             projectID,
		instanceName:          instanceName,
		zone:                  zone,
		sshPublicKeyFileName:  sshPublicKeyFileName,
		sshPrivateKeyFileName: sshPrivateKeyFileName,
		sshKeyUsername:        sshKeyUsername,
		customEveNGImageName:  customEveNGImageName,
	}

	service, err := initService(ctx)
	if err != nil {
		return nil, err
	}

	client.service = service

	return client, nil
}

func initService(ctx context.Context) (*compute.Service, error) {
	client, err := google.DefaultClient(ctx, compute.ComputeScope)
	if err != nil {
		return nil, err
	}
	service, err := compute.New(client)
	if err != nil {
		return nil, err
	}
	return service, err
}

func cronstructFirewallRules(direction string) *compute.Firewall {
	rule := &compute.Firewall{
		Kind:      "compute#firewall",
		Name:      strings.ToLower(direction) + "-eve",
		SelfLink:  "projects/amb1s1/global/firewalls/" + strings.ToLower(direction) + "-eve",
		Network:   "projects/amb1s1/global/networks/default",
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
	prefix := "https://www.googleapis.com/compute/v1/projects/" + c.projectID
	sshKey := c.readSSHKey()

	instance := &compute.Instance{
		Name:           c.instanceName,
		Description:    "compute sample instance",
		MinCpuPlatform: "Intel Cascade Lake",
		MachineType:    prefix + "/zones/" + c.zone + "/machineTypes/c2-standard-4",
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
					DiskName:    "my-root-" + c.instanceName,
					SourceImage: "projects/amb1s1/global/images/" + c.customEveNGImageName,
					DiskType:    "projects/amb1s1/zones/us-central1-a/diskTypes/pd-ssd",
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
					Value: proto.String(c.sshKeyUsername + ":" + string(sshKey)),
				},
			},
		},
	}
	return instance

}

func (c *Client) isInstanceExists() bool {
	found, _ := c.service.Instances.Get(c.projectID, c.zone, c.instanceName).Do()
	if found != nil {
		return true
	}
	return false

}

func (c *Client) NATIPLookup() (net.Addr, error) {
	time.Sleep(60 * time.Second)
	log.Println("looking for the instance NatIP")
	found, _ := c.service.Instances.Get(c.projectID, c.zone, c.instanceName).Do()
	for _, i := range found.NetworkInterfaces {
		nat, err := net.ResolveIPAddr("ip", i.AccessConfigs[len(i.AccessConfigs)-1].NatIP)
		if err != nil {
			return nil, err
		}
		return nat, nil
	}
	return nil, nil
}

func (c *Client) createFirewallRule(direction string) error {
	firewallRequest := cronstructFirewallRules(direction)
	op, err := c.service.Firewalls.Insert(c.projectID, firewallRequest).Do()
	if err != nil {
		return err
	}
	log.Println(op.Description)
	return nil

}

func (c *Client) CreateFirewallRules() error {
	for _, d := range []string{"INGRESS", "EGRESS"} {
		err := c.createFirewallRule(d)
		if err != nil {
			return err
		}
	}
	return nil

}
func (c *Client) CreateInstance() error {
	exists := c.isInstanceExists()
	if exists {
		return errors.New("Instance " + c.instanceName + " already exists.")
	}
	instanceRequest := c.contructInstanceRequest()
	op, err := c.service.Instances.Insert(c.projectID, c.zone, instanceRequest).Do()
	if err != nil {
		return err
	}
	etag := op.Header.Get("Etag")
	_, err = c.service.Instances.Get(c.projectID, c.zone, c.instanceName).IfNoneMatch(etag).Do()
	if err != nil {
		return err
	}
	if googleapi.IsNotModified(err) {
		log.Printf("Instance not modified since insert.")
	} else {
		log.Printf("Instance modified since insert.")
	}
	return nil
}

func (c *Client) CreateEveNGImage() {
	image := &compute.Image{
		Name: c.customEveNGImageName,
		Licenses: []string{
			"https://www.google.com/compute/v1/projects/vm-options/global/licenses/enable-vmx",
		},
		SourceDisk: "https://www.googleapis.com/compute/beta/projects/ubuntu-os-cloud/global/images/ubuntu-1604-xenial-v20210429",
		DiskSizeGb: 10,
	}
	_, err := c.service.Images.Insert(c.projectID, image).Do()
	if err != nil {
		log.Fatalf("failed to create a new image error: %v", err)
	}
}

func (c *Client) readSSHKey() []byte {
	body, err := ioutil.ReadFile(c.sshPublicKeyFileName)
	if err != nil {
		log.Fatalf("could not read public ssh file, error: %v", err)
	}
	return body
}

func (c *Client) sshToServer(natIP net.Addr) *goph.Client {
	log.Printf("ssh to: %v", natIP)
	// Start new ssh connection with private key.
	priKey, err := goph.Key(c.sshPrivateKeyFileName, "")
	if err != nil {
		log.Fatalf("could not get the ssh private key, error: %v", err)
	}

	client, err := goph.NewUnknown(c.sshKeyUsername, natIP.String(), priKey)
	if err != nil {
		log.Fatalf("could not create a new ssh client, error: %v", err)
	}
	return client

}

func (c *Client) fetchScript(client *goph.Client) error {
	dir, _ := os.Getwd()
	for _, f := range bashFiles {
		err := client.Upload(dir+"/"+f, "/home/"+c.sshKeyUsername+"/"+f)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) runRemoteScript(client *goph.Client, f string) error {
	// Execute your command.
	log.Printf("runnning script on file %v ...", f)
	out, err := client.Run("chmod +x /home/" + c.sshKeyUsername + "/" + f)
	if err != nil {
		return err
	}
	log.Println(string(out))
	out, err = client.Run("sudo /home/" + c.sshKeyUsername + "/" + f)
	if err != nil {
		return err
	}
	log.Println(string(out))

	return nil
}

func reboot(client *goph.Client) error {
	log.Println("rebooting....")
	out, err := client.Run("sudo reboot -f")
	log.Printf("Reboot Out: %v", string(out))
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) InitialSetup(natIP net.Addr) error {
	for _, f := range bashFiles {
		log.Printf("fetching file %v to remote server %v", f, natIP.String())
		client := c.sshToServer(natIP)
		defer client.Close()
		err := c.fetchScript(client)
		if err != nil {
			log.Println(err)
		}
		err = c.runRemoteScript(client, f)
		if err != nil {
			return err
		}
		reboot(client)
		time.Sleep(60 * time.Second)
	}
	return nil
}
