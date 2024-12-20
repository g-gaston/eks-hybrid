#!/usr/bin/env bash
# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

CILIUM_VERSION=$1

OPERATOR_DIGEST=$(docker buildx imagetools inspect 381492195191.dkr.ecr.us-west-2.amazonaws.com/cilium/operator-generic:$CILIUM_VERSION --format '{{json .Manifest.Digest}}')
CILIUM_DIGEST=$(docker buildx imagetools inspect 381492195191.dkr.ecr.us-west-2.amazonaws.com/cilium/cilium:$CILIUM_VERSION --format '{{json .Manifest.Digest}}')

cat <<EOF > ./cilium-values.yaml
affinity:
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
      - matchExpressions:
        - key: eks.amazonaws.com/compute-type
          operator: In
          values:
          - hybrid
operator:
  image:
    repository: "381492195191.dkr.ecr.us-west-2.amazonaws.com/cilium/operator"
    tag: "$CILIUM_VERSION"
    imagePullPolicy: "IfNotPresent"
    digest: $OPERATOR_DIGEST
  replicas: 1
  unmanagedPodWatcher:
    restart: false
ipam:
  mode: cluster-pool
envoy:
  enabled: false
image:
  repository: "381492195191.dkr.ecr.us-west-2.amazonaws.com/cilium/cilium"
  tag: "$CILIUM_VERSION"
  imagePullPolicy: "IfNotPresent"
  digest: $CILIUM_DIGEST
preflight:
  image:
    repository: "381492195191.dkr.ecr.us-west-2.amazonaws.com/cilium/cilium"
    tag: "$CILIUM_VERSION"
    imagePullPolicy: "IfNotPresent"
    digest: $CILIUM_DIGEST
EOF

helm template cilium cilium/cilium --version ${CILIUM_VERSION:1} --namespace kube-system --values ./cilium-values.yaml --set ipam.operator.clusterPoolIPv4PodCIDRList='\{\{.PodCIDR\}\}' >  ./cilium-template.yaml

echo "$CILIUM_VERSION" > VERSION
