// Copyright 2017 Google LLC.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go-eve/goeve"
)

var (
	instanceName           = flag.String("instance_name", "", "name of your compute instance")
	configFile             = flag.String("config_file", "config.yaml", "absolute path to the goeve config file")
	createCustomEveNGImage = flag.Bool("create_custom_eve_ng_image", false, "Create a custom eve-ng image if not already created")
	resetInstance          = flag.Bool("reset_instance", false, "if true, the too will delete and rebuild the instance")
)

func main() {
	flag.Parse()
	out := goeve.Run(*instanceName, *configFile, *createCustomEveNGImage, *resetInstance)
	s, _ := json.MarshalIndent(out, "", "\t")
	fmt.Print(string(s))

}
