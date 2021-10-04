package goeve

import (
	"errors"
	"io/ioutil"
	"log"
	"net"
	"strings"
	"time"

	"github.com/amb1s1/go-eve/connect"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v2"

	evecompute "github.com/amb1s1/go-eve/eve-compute"
	compute "google.golang.org/api/compute/v1"
)

var (
	bashFiles    = []string{"install.sh", "eve-initial-setup.sh"}
	configFile   = "config.yaml"
	fwDirections = []string{"INGRESS", "EGRESS"}
)

type firewalls struct {
	Ingress string
	Egress  string
}

// Status is the representation of the final tool run state.
type Status struct {
	Instance string
	Settings string
	Firewall firewalls
}

type client struct {
	ProjectID         string `yaml:"projectID"`
	InstanceName      string `yaml:"instanceName"`
	Zone              string `yaml:"zone"`
	PublicKeyPath     string `yaml:"publicKeyPath"`
	PrivateKeyPath    string `yaml:"privateKeyPath"`
	SSHKeyUsername    string `yaml:"sshKeyUsername"`
	CustomImageName   string `yaml:"customImageName"`
	MachineType       string `yaml:"machineType"`
	DiskSize          int64  `yaml:"diskSize"`
	createCustomImage bool
	Status            *Status
}

func new(instanceName, orideConfigFile string, createCustomImage bool) (*client, error) {
	c := &client{}

	if orideConfigFile != "" {
		configFile = orideConfigFile
	}

	f, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatalf("Could not read public ssh file error: %v", err)
	}

	if err := yaml.Unmarshal(f, &c); err != nil {
		return nil, err
	}

	if instanceName != "" {
		c.InstanceName = instanceName
	}

	c.createCustomImage = createCustomImage

	return c, nil
}

func (c *client) firewallRequest(direction string) *compute.Firewall {
	log.Printf("Constructing the firewall rule %v", direction)

	r := &compute.Firewall{
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
		r.DestinationRanges = append(r.DestinationRanges, "0.0.0.0/0")
		return r
	}

	r.SourceRanges = append(r.SourceRanges, "0.0.0.0/0")

	return r
}

func (c *client) instanceRequest() *compute.Instance {
	log.Println("Constructing instance request")

	prefix := "https://www.googleapis.com/compute/v1/projects/" + c.ProjectID
	sshKey := c.readSSHKey()

	r := &compute.Instance{
		Name:           c.InstanceName,
		Description:    "eve-ng compute instance created by go-eve",
		MinCpuPlatform: "Intel Cascade Lake",
		MachineType:    prefix + "/zones/" + c.Zone + "/machineTypes/" + strings.ToLower(c.MachineType),
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
					SourceImage: "projects/" + c.ProjectID + "/global/images/" + c.CustomImageName,
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
	return r

}

func (c *client) imageRequest() *compute.Image {
	log.Println("Constructing image request")

	r := &compute.Image{
		Name: c.CustomImageName,
		Licenses: []string{
			"https://www.google.com/compute/v1/projects/vm-options/global/licenses/enable-vmx",
		},
		SourceImage: "https://www.googleapis.com/compute/beta/projects/ubuntu-os-cloud/global/images/ubuntu-1604-xenial-v20210429",
		DiskSizeGb:  c.DiskSize,
	}

	return r
}

func (c *client) readSSHKey() []byte {
	log.Println("Reading ssh key")

	f, err := ioutil.ReadFile(c.PublicKeyPath)
	if err != nil {
		log.Fatalf("Could not read public ssh file %v, error: %v", c.PublicKeyPath, err)
	}

	return f

}

func (c *client) initialSetup(publicKey, privateKey, username string, ip net.Addr) error {
	log.Println("Initializing eve-go settings")

	for _, f := range bashFiles {
		sc, err := connect.NewClient(publicKey, privateKey, username, ip)
		if err != nil {
			return err
		}

		if err := sc.Fetch(f); err != nil {
			return err
		}

		out, err := sc.RunScript(f)
		if err != nil {
			return err
		}

		if string(out) == "VM is alredy configured\n" {
			log.Println(strings.ToLower(string(out)))
			c.Status.Settings = "not modified"

			return nil
		}
		time.Sleep(60 * time.Second)
		if err := sc.Reboot(); err != nil {
			return err
		}
	}

	c.Status.Settings = "configured"

	return nil
}

func (c *client) createImage(s evecompute.ServiceFunctions) error {
	r := c.imageRequest()

	if imageCreated := s.IsImageCreated(c.ProjectID, c.CustomImageName); !imageCreated { // image not created
		if err := s.CreateImage(c.ProjectID, r); err != nil {
			return err
		}

		return nil
	}

	log.Printf("Custom image name: %v is already created. Skipping new custom image creation.", c.CustomImageName)

	return nil
}

func (c *client) createInstance(s evecompute.ServiceFunctions) error {
	r := c.instanceRequest()

	if err := s.CreateInstance(c.ProjectID, c.Zone, r); err != nil {
		return err
	}

	return nil
}

func (c *client) createFirewallRules(s evecompute.ServiceFunctions) error {
	for _, f := range fwDirections {
		fr := c.firewallRequest(f)

		if err := s.InsertFirewallRule(c.ProjectID, fr); err != nil {

			if f == "INGRESS" {
				c.Status.Firewall.Ingress = "not modified"
			}

			if f == "EGRESS" {
				c.Status.Firewall.Egress = "not modified"
			}

			return err
		}
	}

	return nil
}

func (c *client) setupInstance(s evecompute.ServiceFunctions) error {
	log.Println("Seting instance")

	ip, err := s.LookupExternalIP(c.ProjectID, c.Zone, c.InstanceName)
	if err != nil {
		return err
	}

	if err := c.initialSetup(c.PublicKeyPath, c.PrivateKeyPath, c.SSHKeyUsername, ip); err != nil {
		return err
	}

	return nil
}

func (c *client) teardown(s evecompute.ServiceFunctions) error {
	if err := s.DeleteInstance(c.ProjectID, c.Zone, c.InstanceName); err != nil {
		return err
	}

	c.Status.Instance = "Deleted"
	c.Status.Settings = "Gone with the Instance"

	if err := s.DeleteFirewallRules(c.ProjectID); err != nil {
		return err
	}

	c.Status.Firewall.Egress = "Deleted"
	c.Status.Firewall.Ingress = "Deleted"

	if err := s.DeleteImage(c.ProjectID, c.CustomImageName); err != nil {
		return err
	}

	return nil
}

func (c *client) resetInstance(s evecompute.ServiceFunctions) error {
	if err := s.DeleteInstance(c.ProjectID, c.Zone, c.InstanceName); err != nil {
		return err
	}

	if err := c.create(s); err != nil {
		return err
	}

	return nil
}

func (c *client) create(s evecompute.ServiceFunctions) error {
	log.Println("Create, start the workflow for creating a new compute instance")

	if c.createCustomImage {
		if err := c.createImage(s); err != nil {
			return err
		}
	}

	if err := c.createInstance(s); err != nil {
		return err
	}

	if err := c.createFirewallRules(s); err != nil {
		return err
	}

	c.Status.Instance = "new instance " + c.InstanceName + " was created"
	if c.Status.Firewall.Ingress == "" {
		c.Status.Firewall.Ingress = "created"
	}

	if c.Status.Firewall.Egress == "" {
		c.Status.Firewall.Egress = "created"
	}

	if status := s.InstanceStatus(c.ProjectID, c.Zone, c.InstanceName); status == "TERMINATED" {
		if err := s.StartInstance(c.ProjectID, c.Zone, c.InstanceName); err != nil {
			return nil
		}
	}

	if err := c.setupInstance(s); err != nil {
		return err
	}

	return nil
}

func (c *client) stop(status string, s evecompute.ServiceFunctions) error {
	if status == "" {
		return errors.New("compute instance does not exists")
	}

	if status == "TERMINATED" {
		return errors.New("compute instance is not running")
	}

	if err := s.StopInstance(c.ProjectID, c.Zone, c.InstanceName); err != nil {
		return nil
	}

	c.Status.Instance = "Stopped"
	c.Status.Firewall.Egress = "Not modified"
	c.Status.Firewall.Ingress = "Not modified"
	c.Status.Settings = "Not modified"

	return nil
}

// Run handler the start of the creation, resetInstance, stop and teardown logic.
func Run(instanceName, configFile string, createCustomImage, createLab, resetInstance, stop, teardown bool) *Status {
	c, err := new(instanceName, configFile, createCustomImage)
	if err != nil {
		log.Fatalf("Could not create a new goeve client, error: %v", err)
	}

	c.Status = &Status{
		Firewall: firewalls{},
	}

	service, err := evecompute.New()
	if err != nil {
		log.Fatalf("Could not create a new compute service, error: %v", err)
	}

	status := service.InstanceStatus(c.ProjectID, c.Zone, c.InstanceName)

	switch {
	case stop:
		if err := c.stop(status, service); err != nil {
			log.Fatalf("Could not stop compute instance %v, error: %v", c.InstanceName, err)
		}
	case resetInstance:
		if err := c.resetInstance(service); err != nil {
			log.Fatalf("Could not reset instance %v, error: %v", c.InstanceName, err)
		}
	case teardown:
		if err := c.teardown(service); err != nil {
			log.Fatalf("Could not teardown lab for compute instance %v, error: %v", c.InstanceName, err)
		}
	case createLab:
		if err := c.create(service); err != nil {
			log.Fatalf("Could not create an entire lab, error: %v", err)
		}
	}

	return c.Status

}
