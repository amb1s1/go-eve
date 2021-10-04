// Package evecompute provides all the logic to bring a eve-ng lab to a cloud service.
//
// This package only support Google cloud for now.
package evecompute

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/briandowns/spinner"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
)

// ServiceFunctions defines the ServiceFunctions operation for cloud service.
type ServiceFunctions interface {
	IsImageCreated(string, string) bool
	CreateImage(string, *compute.Image) error
	DeleteImage(string, string) error
	CreateInstance(string, string, *compute.Instance) error
	InsertFirewallRule(string, *compute.Firewall) error
	DeleteFirewallRules(string) error
	LookupExternalIP(string, string, string) (net.Addr, error)
	InstanceStatus(string, string, string) string
	DeleteInstance(string, string, string) error
	StopInstance(string, string, string) error
	StartInstance(string, string, string) error
}

type computeService struct {
	service *compute.Service
}

// New handles the creation of a new cloud service api client.
// For now we only support google cloud.
func New() (ServiceFunctions, error) {
	cs := computeService{}
	ctx := context.Background()

	service, err := initService(ctx)
	if err != nil {
		return nil, err
	}

	cs.service = service

	return cs, nil
}

func initService(ctx context.Context) (*compute.Service, error) {
	c, err := google.DefaultClient(ctx, compute.ComputeScope)

	if err != nil {
		return nil, err
	}

	service, err := compute.New(c)
	if err != nil {
		return nil, err
	}

	return service, err
}

// IsImageCreated verifies if the image is created.
func (c computeService) IsImageCreated(projectID, name string) bool {
	op, err := c.service.Images.Get(projectID, name).Do()
	if err != nil {
		return false
	}

	if op.Status == "READY" {
		return true
	}

	return false
}

// CreatesImage handles the creation of a custom image.
func (c computeService) CreateImage(projectID string, image *compute.Image) error {
	log.Printf("creating new image %v.", image.Name)

	_, err := c.service.Images.Insert(projectID, image).Do()
	if err != nil {
		return err
	}

	for {
		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond) // Build our new spinner
		s.Start()
		time.Sleep(10 * time.Second)
		s.Stop()

		op, _ := c.service.Images.Get(projectID, image.Name).Do()

		log.Printf("creating new image %v status is: %v.", image.Name, op.Status)

		if op.Status != "PENDING" {
			break
		}
	}

	return nil
}

// DeleteImage deletes a custom image.
func (c computeService) DeleteImage(projectID, name string) error {
	if created := c.IsImageCreated(projectID, name); created {
		_, err := c.service.Images.Delete(projectID, name).Do()
		if err != nil {
			return err
		}

		log.Printf("deleted image: %v.", name)

		return nil
	}
	log.Printf("image: %v was not deleted. Image not found.", name)

	return nil
}

// CreateInstance creates a google cloud compute instance.
func (c computeService) CreateInstance(projectID, zone string, request *compute.Instance) error {
	log.Printf("creating instance %v.", request.Name)

	if status := c.InstanceStatus(projectID, zone, request.Name); status == "" {
		_, err := c.service.Instances.Insert(projectID, zone, request).Do()
		if err != nil {
			return err
		}

		for {
			s := spinner.New(spinner.CharSets[9], 100*time.Millisecond) // Build our new spinner
			s.Start()                                                   // Start the spinner
			time.Sleep(8 * time.Second)                                 // Run for some time to simulate work
			s.Stop()

			if status := c.InstanceStatus(projectID, zone, request.Name); status == "RUNNING" {
				return nil
			}
		}
	}

	log.Printf("compute instance %v already exist.", request.Name)

	return nil
}

func isFirewallRuleExist(projectID, name string, c computeService) bool {
	_, err := c.service.Firewalls.Get(projectID, name).Do()
	if err != nil {
		return false
	}

	return true
}

// InsertFirewallRule inserts a file rule into the google cloud project.
func (c computeService) InsertFirewallRule(projectID string, request *compute.Firewall) error {
	if created := isFirewallRuleExist(projectID, request.Name, c); !created {
		_, err := c.service.Firewalls.Insert(projectID, request).Do()
		if err != nil {
			return err
		}
	}

	log.Printf("firewall rule %v already exist.", request.Name)

	return nil
}

// DeleteFirewallRules deletes both ingress and egress firewall rules created by go-eve tool.
func (c computeService) DeleteFirewallRules(projectID string) error {
	for _, f := range []string{"ingress-eve", "egress-eve"} {

		if exist := isFirewallRuleExist(projectID, f, c); exist {
			log.Printf("deleting firewall rule: %v.", f)
			_, err := c.service.Firewalls.Delete(projectID, f).Do()
			if err != nil {
				return err
			}
			log.Printf("deleted firewall rule: %v.", f)
			break

		}

		log.Printf("firewall rule: %v was not deleted. Rule not found.", f)

	}

	return nil
}

// LookupExternalIP looks for the compute instance assigned external ip also know as nat ip.
func (c computeService) LookupExternalIP(projectID, zone, instanceName string) (net.Addr, error) {
	log.Println("looking for the instance external ip.")

	found, _ := c.service.Instances.Get(projectID, zone, instanceName).Do()

	for _, i := range found.NetworkInterfaces {
		ip, err := net.ResolveIPAddr("ip", i.AccessConfigs[len(i.AccessConfigs)-1].NatIP)
		if err != nil {
			return nil, err
		}
		log.Printf("got external ip %v for the compute instance %v.", ip, instanceName)

		return ip, nil
	}

	return nil, nil
}

// InstanceStatus looks for the compute instance status.
func (c computeService) InstanceStatus(projectID, zone, name string) string {
	found, _ := c.service.Instances.Get(projectID, zone, name).Do()
	if found != nil {
		return found.Status
	}

	return ""
}

// DeleteInstance deletes the compute instance.
func (c computeService) DeleteInstance(projectID, zone, name string) error {
	if s := c.InstanceStatus(projectID, zone, name); s != "" {
		_, err := c.service.Instances.Delete(projectID, zone, name).Do()
		if err != nil {
			return err
		}

		for {
			log.Printf("deleting instance %v.", name)
			s := spinner.New(spinner.CharSets[9], 100*time.Millisecond) // Build our new spinner
			s.Start()                                                   // Start the spinner
			time.Sleep(8 * time.Second)                                 // Run for some time to simulate work
			s.Stop()
			if status := c.InstanceStatus(projectID, zone, name); status == "" {
				return nil
			}
		}

	}

	log.Printf("compute instance %v was not deleted. Instance was not found.", name)

	return nil
}

// StopInstance stops/shutdown the compute instance.
func (c computeService) StopInstance(projectID, zone, name string) error {
	status := c.InstanceStatus(projectID, zone, name)

	if status == "RUNNING" {
		_, err := c.service.Instances.Stop(projectID, zone, name).Do()
		if err != nil {
			return err
		}
		for {
			log.Printf("stopping instance %v.", name)
			s := spinner.New(spinner.CharSets[9], 100*time.Millisecond) // Build our new spinner
			s.Start()                                                   // Start the spinner
			time.Sleep(8 * time.Second)                                 // Run for some time to simulate work
			s.Stop()
			if status := c.InstanceStatus(projectID, zone, name); status == "TERMINATED" {
				log.Printf("instance %v stopped.", name)
				return nil
			}
		}

	}

	if status == "" {
		log.Printf("Compute instance %v was not stop. Instance was not found.", name)
		return nil

	}

	log.Printf("Compute instance %v was not stop. Instance is already shutdown.", name)
	return nil
}

// StartInstance starts/turn on the compute instance.
func (c computeService) StartInstance(projectID, zone, name string) error {
	_, err := c.service.Instances.Start(projectID, zone, name).Do()
	if err != nil {
		return err
	}

	for {
		log.Printf("Starting instance %v", name)

		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond) // Build our new spinner
		s.Start()                                                   // Start the spinner
		time.Sleep(8 * time.Second)                                 // Run for some time to simulate work
		s.Stop()

		if status := c.InstanceStatus(projectID, zone, name); status == "RUNNING" {
			log.Printf("Instance %v is running", name)
			break
		}
	}

	return nil
}
