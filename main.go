// Copyright 2017 Google LLC.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/amb1s1/go-eve/goeve"
)

var (
	instanceName      = flag.String("instance_name", "", "name of your compute instance")
	configFile        = flag.String("config_file", "config.yaml", "absolute path to the goeve config file")
	createCustomImage = flag.Bool("create_custom_image", false, "Create a custom eve-ng image if not already created")
	resetInstance     = flag.Bool("reset_instance", false, "if true, the compute instance will delete and rebuild the instance")
	create            = flag.Bool("create", true, "create a lab, default to true")
	stop              = flag.Bool("stop", false, "if true, the compute instance will be shutdown")
	teardown          = flag.Bool("teardown", false, "teardown - delete compute instance, remove firewall and delete custom image.")
)

func main() {
	flag.Parse()
	out := goeve.Run(*instanceName, *configFile, *createCustomImage, *create, *resetInstance, *stop, *teardown)
	s, _ := json.MarshalIndent(out, "", "\t")
	fmt.Print(string(s))

}
