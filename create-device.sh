#!/bin/bash

# TODO: Script should become part of golang executable. This is a crutch!

set -euo pipefail

if [ -z ${DATA_PATH+x} ];then
  echo "ERROR:  DATA_PATH is not set! This must be set in order to run this script"
  exit -1
fi

readonly CERTS_DIR=${DATA_PATH}/certs
readonly DEVICES_DIR=${DATA_PATH}/devices
readonly NAMESPACE=${NAMESPACE:-default}


new_client() {
  UUID=$1
  DEVICE_ID=$2

  if [ -z $UUID ];then
    echo "Must pass in UUID as 1st parameter!"
    exit -1
  fi
  if [ -z $DEVICE_ID ];then
    echo "Must pass in DEVICE_ID as 2nd parameter!"
    exit -1
  fi

  export DEVICE_UUID=$UUID
  local device_id=$DEVICE_ID
  local device_dir="${DEVICES_DIR}/${DEVICE_UUID}"
  
  mkdir -p "${device_dir}"
  echo ""
  echo "=== New Device Creation ==="
  echo "UUID for device = ${DEVICE_UUID}"
  echo "device_id = ${device_id}"

  # This is a tag for including a chunk of code in the docs. Don't remove. tag::genclientkeys[]
  openssl ecparam -genkey -name prime256v1 | openssl ec -out "${device_dir}/pkey.ec.pem"
  openssl pkcs8 -topk8 -nocrypt -in "${device_dir}/pkey.ec.pem" -out "${device_dir}/pkey.pem"
  openssl req -new -config "${CERTS_DIR}/client.cnf" -key "${device_dir}/pkey.pem" -out "${device_dir}/${device_id}.csr"
  openssl x509 -req -days 365 -extfile "${CERTS_DIR}/client.ext" -in "${device_dir}/${device_id}.csr" \
    -CAkey "${DEVICES_DIR}/ca.key" -CA "${DEVICES_DIR}/ca.crt" -CAcreateserial -out "${device_dir}/client.pem"
  cat "${device_dir}/client.pem" "${DEVICES_DIR}/ca.crt" > "${device_dir}/${device_id}.chain.pem"
  openssl x509 -in "${device_dir}/client.pem" -text -noout
  # end::genclientkeys[]
}


new_client $1 $2
