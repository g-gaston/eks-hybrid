#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

source /helpers.sh

mock::aws
wait::dbus-ready

export AWS_ENDPOINT_URL=http://localhost:5000

mkdir -p /etc/iam/pki
touch  /etc/iam/pki/server.pem
touch  /etc/iam/pki/server.key

nodeadm install 1.30  --credential-provider iam-ra

mock::aws_signing_helper

exit_code=0
STDERR=$(nodeadm init --skip run,node-ip-validation,k8s-endpoint-network-validation,k8s-authentication-validation --config-source file://config.yaml 2>&1) || exit_code=$?
if [ $exit_code -ne 0 ]; then
    assert::is-substring "$STDERR" "ResourceNotFoundException"
else
    echo "nodeadm init should have failed while cluster does not exist"
    exit 1
fi

aws eks create-cluster \
    --name my-cluster \
    --region us-west-2 \
    --kubernetes-version 1.31 \
    --role-arn arn:aws:iam::123456789012:role/eksClusterRole-12-3 \
    --resources-vpc-config subnetIds=subnet-123456789012,subnet-123456789013,securityGroupIds=sg-123456789014,endpointPrivateAccess=true,endpointPublicAccess=false \
    --remote-network-config '{"remoteNodeNetworks":[{"cidrs":["10.100.0.0/16"]}],"remotePodNetworks":[{"cidrs":["10.101.0.0/16"]}]}'

if ! nodeadm init --skip run,node-ip-validation,k8s-endpoint-network-validation,k8s-authentication-validation --config-source file://config.yaml; then
    echo "nodeadm init should have succeeded after creating the cluster"
    exit 1
fi
