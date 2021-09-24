// Copyright 2017 Google LLC.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"go-eve/goeve"
)

var (
	instanceName           = flag.String("instance_name", "", "name of your compute instance")
	configFile             = flag.String("config_file", "config.yaml", "absolute path to the goeve config file")
	createCustomEveNGImage = flag.Bool("create_custom_eve_ng_image", false, "Create a custom eve-ng image if not already created")
)

func main() {
	flag.Parse()
	goeve.Run(*instanceName, *configFile, *createCustomEveNGImage)
}
