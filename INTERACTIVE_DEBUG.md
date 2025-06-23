you are running in the dev instance, where the deployment process runs.

You can read the deployment log in the file `deployment.log`. It contains the bastion IP.

To ssh into the bastion use (change to the correct IP):

ssh -i /home/shellbox/.ssh/id_rsa shellbox@4.155.193.6

the bastion logs are in the file `server.log` in the beastion home (/home/shellbox/)

From the bastion, you may ssh into the box instances, these are the ones hosting the qemu vms. Use this to ssh into the instances (use the correct IP from the log, usually it is 10.1.0.4 for the first one):

ssh shellbox@10.1.0.4

from the bastion you may also try to log into the qemu vm if the ssh there is ready:

ssh -p 2222 ubuntu@10.1.0.4

the cloud-init logs of making the golden image are in the bastion in `/var/log/cloud-init-output.log`

the instance of the qemu vm has some logs in the mounted volume in user-data folder, starting with qemu*.log

you can also use the "az" cli from the dev instance to debug Azure resources, the bastion, the boxes everything.

also you can search the wb as if needed.

Use the information above to help the user interactively debug. Note that some logs can be large so maybe grep, sed or start reading them from the end. Also some logs may be being created, as this is all interactive, things are hgapenning. YUou may sleep a bit and then check their status again if needed. You can also check in the code where the logging happens to correlate too the logic.

