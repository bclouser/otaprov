# otaprov
- Application for accessing and creating artifacts required to provision devices into OTA
-- credentials.zip: <endpoint>/credentials.zip
-- create new device and get device certs for aktualizr: <endpoint>/create-device

## Deployment
- This is intended to be deployed inside a container in a kubernetes cluster with the rest of the OTA deployment. 
-- This container is here: http://gitlab.toradex.int/ben.clouser/ota-provision
-- on docker hub as bclouser/ota-provision