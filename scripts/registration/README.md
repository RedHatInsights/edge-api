To register an edge device...

1. Replace YOUR_IP_ADDRESS(ES)_HERE with one or more device DNS names or IPs in the example inventory.ini file.

```
[all]
YOUR_IP_ADDRESS(ES)_HERE
```
e.g.,
```
[all]
192.168.1.100
192.168.1.110
192.168.1.230
```

2. Replace PLACEHOLDERS with RHSM and Insights username/password in the example vars_register_edge_device.yml file.

```
---
# VARS FOR RHSM REGISTER EXAMPLE

rhsm_username: MY_RHSM_USERNAME
rhsm_password: MY_RHSM_PASSWORD
rhc_username: MY_RHC_USERNAME
rhc_password: MY_RHC_PASSWORD
```

3. Execute the playbook with the username provisioned on the device(s).
```
ansible-playbook -i inventory.ini -vv register_edge_device.yml \
--extra-vars="regfile=vars_register_edge_device.yml" --user <THE USER>
```
e.g.,
```
ansible-playbook -i inventory.ini -vv register_edge_device.yml \
--extra-vars="regfile=vars_register_edge_device.yml" --user joe_user
```
