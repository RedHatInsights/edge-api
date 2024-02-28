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


#FIX THE RHCD_T semanage
%post --log=/var/log/anaconda/permissive-rhcd_t.log
/usr/sbin/semanage permissive --add rhcd_t
/usr/sbin/semanage permissive --add insights_client_t
%end

#CUSTOM_POST_HERE
%include /tmp/fleet_kspost.txt


%post --log=/var/log/anaconda/post-autoregister.log
echo POST-AUTOREGISTER

# Automatically register if credentials are provided
[[ -e /root/fleet_env.bash ]] && source /root/fleet_env.bash
RHC_FIRSTBOOT=${RHC_FIRSTBOOT:-false}

# CREATE AUTOREGISTER SCRIPT
# TODO: rhc firstboot registration script should be something installed with RHC (if not already)
cat << '__RHCREGISTER__' >> /usr/local/bin/rhc_autoregister.sh
#!/bin/bash

if [ -e /root/fleet_env.bash ]
then
	source /root/fleet_env.bash

	[[ -e /root/fleet_tags.yaml ]] && cp /root/fleet_tags.yaml /etc/insights-client/tags.yaml

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
		systemctl status insights-client

		# Set specific display name set in custom post
		if [ -z ${INSIGHTS_DISPLAY_NAME+x} ]
		then
			# Replace localhost with Subscription Manager ID and set Insights display name
			# Subscription Manager ID was chosen based on availability. Refactor based on feedback
			statichostname=$(hostnamectl | grep "Static hostname" | awk -F": " '{print $2}')
			transienthostname=$(hostnamectl | grep "Transient hostname" | awk -F": " '{print $2}')
			[[ -z ${transienthostname+x} ]] && displayname=${statichostname} || displayname=${transienthostname}
			if [ $displayname == "localhost.localdomain" ]
			then
				displayname=$(subscription-manager identity | grep "system identity" | awk -F": " '{print $2}')
				insights-client --display-name "${DISPLAY_NAME_PREFIX}${displayname}"
			fi
		else
			insights-client --display-name "$INSIGHTS_DISPLAY_NAME"
		fi
	fi
else
	echo "INFO: No /root/fleet_env.bash file. Skipping registration"
fi
__RHCREGISTER__

# need to make it executable and restore selinux context
chmod 755 /usr/local/bin/rhc_autoregister.sh
restorecon -rv /usr/local/bin

# CREATE AUTO REGISTRATION FIRSTBOOT SERVICE
cat << '__RHCFIRSTBOOTSERVICE__' >> /etc/systemd/system/rhc_autoregister.service
[Unit]
Before=systemd-user-sessions.service
Wants=network-online.target
After=network-online.target
ConditionPathExists=/root/fleet_env.bash

[Service]
Type=oneshot
ExecStart=/usr/local/bin/rhc_autoregister.sh
ExecStartPost=/usr/bin/rm /root/fleet_env.bash
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target

__RHCFIRSTBOOTSERVICE__

# Set up first boot registration or do it now before reboot
[[ $RHC_FIRSTBOOT == "true" ]] \
    && systemctl enable rhc_autoregister.service \
    || /usr/local/bin/rhc_autoregister.sh

%end


%post --log=/var/log/anaconda/post-cleanup.log
# Cleanup fleet-ification
echo POST-CLEANUP

[[ -e /root/fleet_env.bash ]] && source /root/fleet_env.bash
RHC_FIRSTBOOT=${RHC_FIRSTBOOT:-false}

# Clean up fleet install file(s)
[[ $RHC_FIRSTBOOT != "true"  && -e /root/fleet_env.bash ]] && rm /root/fleet_env.bash

%end
