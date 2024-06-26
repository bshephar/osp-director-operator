Q: WHAT IS TESTED HERE?

A:
- Creates:
  * OpenStackNetConfig
  * OpenStackControlPlane
  * OpenStackBaremetalSet
  * OpenStackBackupRequest (in "save" mode)
- Checks that the OpenStackBackupRequest starts in quiescing state
- Once quiesced, checks that the OpenStackBackupRequest ends in the saved state
- Checks for the existence of a corresponding OpenStackBackup with the proper resources within its spec
- Creates an OpenStackBackupRequest (in "cleanRestore" mode) targeting the created OpenStackBackup for restoration
- Checks that the "cleanRestore" cleans all OSP-D operator resources
- Checks that the new OpenStackBackupRequest enters the "Loading" state
- Checks that the new OpenStackBackupRequest reaches the "Restored" state

NOTE: This test assumes you have two (and only two!) extra OCP workers allocated to your cluster as
      BaremetalHosts, that are not in-use as actual cluster nodes
