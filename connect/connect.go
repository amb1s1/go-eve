package connect

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/briandowns/spinner"
	"github.com/melbahja/goph"
)

type connectFunctons interface {
	Fetch(string) error
	RunScript(string) ([]byte, error)
	Reboot() error
}

type Client struct {
	ip         net.Addr
	publicKey  string
	privateKey string
	username   string
	Service    *goph.Client
}

func NewClient(publicKey, privateKey, username string, ip net.Addr) (connectFunctons, error) {
	service, err := Connect(privateKey, username, ip)
	if err != nil {
		return nil, err
	}
	c := Client{
		publicKey:  publicKey,
		privateKey: privateKey,
		username:   username,
		ip:         ip,
		Service:    service,
	}

	return c, nil
}
func Connect(privateKey, username string, ip net.Addr) (*goph.Client, error) {
	// Start new ssh connection with private key.
	priKey, err := goph.Key(privateKey, "")
	if err != nil {
		return nil, fmt.Errorf("could not get privateKey: %v error: %v", privateKey, err)
	}
	rCount := 0
	for {
		log.Printf("ssh to: %v", ip)
		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond) // Build our new spinner
		s.Start()
		time.Sleep(20 * time.Second)
		s.Stop()
		client, err := goph.NewUnknown(username, ip.String(), priKey)
		if err != nil {
			rCount += 1
		} else {
			log.Printf("connected to: %v", ip.String())
			return client, nil
		}
		if rCount >= 3 {
			return nil, fmt.Errorf("could not connect to %v, error: %v", ip.String(), err)
		}
	}
}

func (c Client) Fetch(file string) error {
	dir, _ := os.Getwd()
	log.Printf("fetching file %v to server %v", file, c.ip.String())
	err := c.Service.Upload(dir+"/"+file, "/home/"+c.username+"/"+file)
	if err != nil {
		return err
	}
	return nil
}

func (c Client) RunScript(file string) ([]byte, error) {
	// Execute your command.
	log.Printf("making %v executable...", file)
	_, err := c.Service.Run("chmod +x /home/" + c.username + "/" + file)
	if err != nil {
		return nil, err
	}
	log.Printf("runnning script on file %v ...", file)
	out, err := c.Service.Run("sudo /home/" + c.username + "/" + file)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (c Client) Reboot() error {
	log.Println("rebooting....")
	out, err := c.Service.Run("sudo reboot -f")
	log.Printf("Reboot Out: %v", string(out))
	if err != nil {
		return err
	}
	return nil
}
