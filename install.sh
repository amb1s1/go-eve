#!/bin/bash
# Check if VM is already configured
if [[ -e /opt/ovf/.configured ]]; then
    echo "VM is already configured"
    exit
fi
# Avoiding grub config gui prompt
sed -i "s/#\ conf_force_conffold=YES/conf_force_conffold=YES/g" /etc/ucf.conf


wget -O - http://www.eve-ng.net/repo/install-eve.sh | bash -i
sudo apt-get update
sudo apt-get -y upgrade
sudo apt-get install dialog
# Avoiding OS to rename eth0 interface to ens4
sudo sed -i 's/GRUB_CMDLINE_LINUX.*$/GRUB_CMDLINE_LINUX="net.ifnames=0 biosdevname=0"/g' /etc/default/grub
sudo sed -i "s/conf_force_conffold=YES/#conf_force_conffold=YES/g" /etc/ucf.conf
