lang en_US.UTF-8
keyboard us
timezone UTC
zerombr
clearpart --all --initlabel
autopart --type=plain --fstype=xfs --nohome
reboot
text
network --bootproto=dhcp --device=link --activate --onboot=on

# Gen the ostreesetup line in the pre section
%include /tmp/ostreesetup


%pre
echo PRE

# RHEL for Edge 8.5 moves the ostree dir to the root of the image
# RHEL for Edge 8.4 and 9.0 install from /run/install/repo
# Auto-detect a dir at that location and inject it into the command list for install
[[ -d /run/install/repo/ostree ]] && repodir='/run/install/repo/ostree/repo' || repodir='/ostree/repo'
ref=$(ostree refs --repo=${repodir})
echo "ostreesetup --nogpg --osname=rhel-edge --remote=rhel-edge --url=file://${repodir} --ref=${ref}" > /tmp/ostreesetup

# Handle include for custom post section if a post file exists
[[ -e /run/install/repo/fleet_kspost.txt ]] && cp /run/install/repo/fleet_kspost.txt /tmp \
	|| echo "#NO CUSTOM POST" > /tmp/fleet_kspost.txt

%end


%pre-install
echo PRE-INSTALL

# Copy the fleet files from the root of the install image
[[ -e /run/install/repo/fleet_env.bash ]] && cp /run/install/repo/fleet_env.bash /tmp \
	|| echo "No fleet_env.bash file to copy"
[[ -e /run/install/repo/fleet_authkeys.txt ]] && cp /run/install/repo/fleet_authkeys.txt /tmp \
	|| echo "No fleet_authkeys.txt file to copy"
[[ -e /run/install/repo/fleet_tags.yaml ]] && cp /run/install/repo/fleet_tags.yaml /tmp \
	|| echo "No fleet_tags.yaml file to copy"

%end


%post --nochroot --log=/mnt/sysroot/var/log/anaconda/post-cpenv.log
echo POST-COPYFILES

# Copy the fleet env file from the /tmp dir to /root on disk
[[ -e /tmp/fleet_env.bash ]] && cp /run/install/repo/fleet_env.bash /mnt/sysroot/root \
	|| "No fleet_env.bash file to copy"
[[ -e /tmp/fleet_authkeys.txt ]] && cp /run/install/repo/fleet_authkeys.txt /mnt/sysroot/root \
	|| "No fleet_authkeys.txt file to copy"
[[ -e /tmp/fleet_tags.yaml ]] && cp /run/install/repo/fleet_tags.yaml /mnt/sysroot/root \
	|| "No fleet_tags.yaml file to copy"

%end


%post --log=/var/log/anaconda/post-user-install.log --erroronfail
echo POST-USER
# Add User and SSH Key provided via UI 
# add user and ssh key
USER_NAME={{.Username}}
useradd -m -G wheel $USER_NAME
USER_HOME=$(getent passwd $USER_NAME | awk -F: '{print $6}')

mkdir -p ${USER_HOME}/.ssh
chown ${USER_NAME}:${USER_NAME} ${USER_HOME}/.ssh
chmod 0700 ${USER_HOME}/.ssh
cat <<'__AUTHKEYS__' >> ${USER_HOME}/.ssh/authorized_keys 
{{.Sshkey}}
__AUTHKEYS__
chmod 600 ${USER_HOME}/.ssh/authorized_keys
chown ${USER_NAME}:${USER_NAME} ${USER_HOME}/.ssh/authorized_keys
# no sudo password for user 
echo -e "${USER_NAME}\tALL=(ALL)\tNOPASSWD: ALL" >> /etc/sudoers

%end


%post --log=/var/log/anaconda/post-user-autoinstall.log
echo POST-USER-AUTOINSTALL

# Create an admin user with authorized_keys
if [ -e /root/fleet_env.bash ]
then
	source /root/fleet_env.bash

	if [ -e /root/fleet_authkeys.txt ]
	then
		# Grab the ADMIN_USER from the first line of the authkeys file
		ADMIN_USER=$(grep ADMIN_USER /root/fleet_authkeys.txt | awk -F= '{print $2}')
		[[ -z $ADMIN_USER ]] && echo "No admin user specified" || useradd -m -G wheel $ADMIN_USER

		USER_HOME=$(getent passwd $ADMIN_USER | awk -F: '{print $6}')
		mkdir -p ${USER_HOME}/.ssh
		chown ${ADMIN_USER}:${ADMIN_USER} ${USER_HOME}/.ssh
		chmod 0700 ${USER_HOME}/.ssh

		cat /root/fleet_authkeys.txt >> ${USER_HOME}/.ssh/authorized_keys
		chmod 600 ${USER_HOME}/.ssh/authorized_keys
		chown ${ADMIN_USER}:${ADMIN_USER} ${USER_HOME}/.ssh/authorized_keys

		# no sudo password for user 
		echo -e "${ADMIN_USER}\tALL=(ALL)\tNOPASSWD: ALL" >> /etc/sudoers
	fi
fi

[[ -e /root/fleet_authkeys.txt ]] && rm /root/fleet_authkeys.txt

%end


%post --log=/var/log/anaconda/insights-on-reboot-unit-install.log --interpreter=/usr/bin/bash --erroronfail
echo POST-INSIGHTS-CLIENT-OVERRIDE
INSIGHTS_CLIENT_OVERRIDE_DIR=/etc/systemd/system/insights-client.service.d
INSIGHTS_CLIENT_OVERRIDE_FILE=$INSIGHTS_CLIENT_OVERRIDE_DIR/override.conf
if [ ! -f $INSIGHTS_CLIENT_OVERRIDE_FILE ]; then
    mkdir -p $INSIGHTS_CLIENT_OVERRIDE_DIR
    cat > $INSIGHTS_CLIENT_OVERRIDE_FILE << EOF 
[Unit]
Requisite=greenboot-healthcheck.service
After=network-online.target greenboot-healthcheck.service osbuild-first-boot.service
[Install]
WantedBy=multi-user.target
EOF
    systemctl enable insights-client.service
fi
%end


#FIX THE RHCD_T semanage
%post --log=/var/log/anaconda/permissive-rhcd_t.log
/usr/sbin/semanage permissive --add rhcd_t
/usr/sbin/semanage permissive --add insights_client_t
%end


#CUSTOM_POST_HERE
%include /tmp/fleet_kspost.txt


%post --log=/var/log/anaconda/post-cleanup.log
# Cleanup fleet-ification
echo POST-CLEANUP

[[ -e /root/fleet_env.bash ]] && source /root/fleet_env.bash
RHC_FIRSTBOOT=${RHC_FIRSTBOOT:-false}

# Clean up fleet install file(s)
[[ $RHC_FIRSTBOOT != "true"  && -e /root/fleet_env.bash ]] && rm /root/fleet_env.bash

%end
