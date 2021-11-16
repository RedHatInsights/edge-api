lang en_US.UTF-8
keyboard us
timezone UTC
zerombr
clearpart --all --initlabel
autopart --type=plain --fstype=xfs --nohome
reboot
text
network --bootproto=dhcp --device=link --activate --onboot=on

# gen the ostreesetup line in pre section
%include /tmp/ostreesetup


%pre
echo PRE
# RHEL for Edge 8.5 moves the ostree dir to the root of the image
# Auto-detect a dir at that location and inject it into the command list for install
# Default to prior ostree/repo location in 8.4
[[ -d /run/install/repo/ostree ]] \
	&& echo "ostreesetup --nogpg --osname=rhel-edge --remote=rhel-edge --url=file:///run/install/repo/ostree/repo --ref=rhel/8/x86_64/edge" > /tmp/ostreesetup \
	|| echo "ostreesetup --nogpg --osname=rhel-edge --remote=rhel-edge --url=file:///ostree/repo --ref=rhel/8/x86_64/edge" > /tmp/ostreesetup

%end


%post --log=/var/log/anaconda/post-install.log --erroronfail
# add user and ssh key
useradd -m -G wheel {{.Username}}
USER_HOME=$(getent passwd {{.Username}} | awk -F: '{print $6}')

mkdir -p ${USER_HOME}/.ssh
chmod 755 ${USER_HOME}/.ssh
tee ${USER_HOME}/.ssh/authorized_keys > /dev/null << STOPHERE
{{.Sshkey}}
STOPHERE
chmod 600 ${USER_HOME}/.ssh/authorized_keys
chown {{.Username}}:{{.Username}} ${USER_HOME}/.ssh/authorized_keys
# no sudo password for user 
echo -e '{{.Username}}\tALL=(ALL)\tNOPASSWD: ALL' >> /etc/sudoers

%end

%post --log=/var/log/anaconda/insights-on-reboot-unit-install.log --interpreter=/usr/bin/bash --erroronfail
INSIGHTS_CLIENT_OVERRIDE_DIR=/etc/systemd/system/insights-client.service.d
INSIGHTS_CLIENT_OVERRIDE_FILE=$INSIGHTS_CLIENT_OVERRIDE_DIR/override.conf

if [ ! -f $INSIGHTS_CLIENT_OVERRIDE_FILE ]; then
    mkdir -p $INSIGHTS_CLIENT_OVERRIDE_DIR
    cat > $INSIGHTS_CLIENT_OVERRIDE_FILE << EOF 
[Unit]
Requisite=greenboot-healthcheck.service
After=network-online.target greenboot-healthcheck.service

[Install]
WantedBy=multi-user.target
EOF

    systemctl enable insights-client.service
fi

%end
