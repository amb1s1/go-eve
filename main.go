// Copyright 2017 Google LLC.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"go-eve/goeve"
)

var (
	projectID              = flag.String("project", "", "name of your project")
	instanceName           = flag.String("instance_name", "", "name of your compute instance")
	zone                   = flag.String("zone", "us-central1-a", "default to us-central1-a zone")
	sshPublicKeyFileName   = flag.String("ssh_public_key_file_name", "", "path to the file containing your ssh public key.")
	sshPrivateKeyFileName  = flag.String("ssh_private_key_file_name", "", "path to the file containing your ssh private key.")
	sshKeyUsername         = flag.String("ssh_key_username", "", "use the username from your ssh public key. If appear on your ssh public key file. If not do not use this flag. E.g user@domain.")
	createCustomEveNGImage = flag.Bool("create_custom_eve_ng_image", false, "Create a custom eve-ng image if not already created")
	customEveNGImageName   = flag.String("custom_eve_ng_image_name", "eve-ng", "Create a custom eve-ng image if not already created. Default is eve-ng")
)

func main() {
	flag.Parse()
	goeve.Run(*projectID, *instanceName, *zone, *sshPublicKeyFileName, *sshPrivateKeyFileName, *sshKeyUsername, *customEveNGImageName, *createCustomEveNGImage)
}
