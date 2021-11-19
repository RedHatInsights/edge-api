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
# Auto-detect a dir at that location and inject it into the command list for install
# Default to prior ostree/repo location in 8.4
[[ -d /run/install/repo/ostree ]] \
	&& echo "ostreesetup --nogpg --osname=rhel-edge --remote=rhel-edge --url=file:///run/install/repo/ostree/repo --ref=rhel/8/x86_64/edge" > /tmp/ostreesetup \
	|| echo "ostreesetup --nogpg --osname=rhel-edge --remote=rhel-edge --url=file:///ostree/repo --ref=rhel/8/x86_64/edge" > /tmp/ostreesetup

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

%end


%post --nochroot --log=/mnt/sysroot/var/log/anaconda/post-cpenv.log
echo POST-COPYFILES

# Copy the fleet env file from the /tmp dir to /root on disk
[[ -e /tmp/fleet_env.bash ]] && cp /run/install/repo/fleet_env.bash /mnt/sysroot/root \
	|| "No fleet_env.bash file to copy"
[[ -e /tmp/fleet_authkeys.txt ]] && cp /run/install/repo/fleet_authkeys.txt /mnt/sysroot/root \
	|| "No fleet_authkeys.txt file to copy"

%end


%post --log=/var/log/anaconda/post-install.log --erroronfail
echo POST-USER
# Add User and SSH Key provided via UI 
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


%post --log=/var/log/anaconda/post-user-autoinstall.log
echo POST-USER-AUTOINSTALL

# Create an admin user with authorized_keys
if [ -e /root/fleet_env.bash ]
then
	source /root/fleet_env.bash

	[[ -z $ADMIN_USER ]] && echo "No admin user specified" || useradd -m -G wheel $ADMIN_USER
	if [ -e /root/fleet_authkeys.txt ]
	then
		USER_HOME=$(getent passwd $ADMIN_USER | awk -F: '{print $6}')
		mkdir -p ${USER_HOME}/.ssh
		chmod 755 ${USER_HOME}/.ssh
		chown ${ADMIN_USER}:${ADMIN_USER} ${USER_HOME}/.ssh

		cat /root/fleet_authkeys.txt >> $USER_HOME/.ssh/authorized_keys
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
After=network-online.target greenboot-healthcheck.service

[Install]
WantedBy=multi-user.target
EOF

    systemctl enable insights-client.service
fi

%end


#CUSTOM_POST_HERE
%include /tmp/fleet_kspost.txt


%post --log=/var/log/anaconda/post-autoregister.log
set -x
echo POST-AUTOREGISTER

# Automatically register if credentials are provided
if [ -e /root/fleet_env.bash ]
then
	source /root/fleet_env.bash

	if [[ -z ${RHC_ORGID+x} ]] && [[ -z ${RHC_USER+x} ]]
	then
		echo "No credentials provided for registration"
	else
		# Register with RHSM
		[[ -v RHC_ORGID ]] \
			&& subscription-manager register --org $RHC_ORGID --activationkey $RHC_ACTIVATION_KEY --force \
			|| subscription-manager register --username $RHC_USER --password $RHC_PASS --auto-attach --force

		# Register with Insights
		insights-client --register > /var/log/anaconda/post-insights-command.log 2>&1

		# Enable and start RHCD service
		systemctl enable rhcd.service
		systemctl restart rhcd.service

		# Register with RHC
		[[ -v RHC_ORGID ]] \
			&& rhc connect --organization $RHC_ORGID --activation-key $RHC_ACTIVATION_KEY \
			|| rhc connect --username $RHC_USER --password $RHC_PASS

		systemctl status rhcd.service

		# Set specific display name set in custom post
		if [ -z ${INSIGHTS_DISPLAY_NAME+x} ]
		then
			# Replace localhost with Machine ID and set Insights display name
			# Machine ID was chosen based on availability. Refactor based on feedback
			statichostname=$(hostnamectl | grep "Static hostname" | awk -F": " '{print $2}')
			transienthostname=$(hostnamectl | grep "Transient hostname" | awk -F": " '{print $2}')
			[[ -z ${transienthostname+x} ]] && displayname=${statichostname} || displayname=${transienthostname}
			if [ $displayname == "localhost.localdomain" ]
			then
				displayname=$(subscription-manager identity | grep "system identity" | awk -F": " '{print $2}')
				insights-client --display-name "${DISPLAY_NAME_PREFIX}_$displayname"
			fi
		else
			insights-client --display-name "$INSIGHTS_DISPLAY_NAME"
		fi
	fi
else
	echo "INFO: No /root/fleet_env.bash file. Skipping registration"
fi

%end


%post --log=/var/log/anaconda/post-cleanup.log
# Cleanup fleet-ification
echo POST-CLEANUP

# Clean up fleet install file(s)
[[ -e /root/fleet_env.bash ]] && rm /root/fleet_env.bash

%end
