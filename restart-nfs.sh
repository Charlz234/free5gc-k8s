#!/bin/bash
# Free5GC NF restart in correct order with deep cleanup

echo "=== Step 0: Host-Level Runtime & CNI Cleanup ==="
# Clean up stale CNI network namespaces and Multus sockets that cause EOF errors
sudo ip -all netns delete 2>/dev/null
sudo rm -rf /var/run/multus/* 2>/dev/null
sudo rm -rf /var/lib/cni/networks/multus-cni-network/* 2>/dev/null

echo "=== Step 1: Clean up stuck pods ==="
kubectl get pods -n free5gc | grep -E "Unknown|Terminating|ContainerCreating|Error" | awk '{print $1}' | \
  xargs -r kubectl delete pod -n free5gc --force --grace-period=0 2>/dev/null

echo "=== Step 2: Clean up stale ReplicaSets ==="
kubectl get rs -n free5gc -o name | while read rs; do
  desired=$(kubectl get $rs -n free5gc -o jsonpath='{.spec.replicas}')
  if [ "$desired" = "0" ]; then
    echo "Deleting stale RS: $rs"
    kubectl delete $rs -n free5gc
  fi
done

echo "=== Step 3: Scale down all NFs ==="
kubectl get deployments -n free5gc -o name | \
  xargs kubectl scale -n free5gc --replicas=0 2>/dev/null || true
sleep 5

echo "=== Step 4: Refresh CNI Daemonsets ==="
# Force Multus to recreate its sockets and links to Cilium
kubectl rollout restart ds kube-multus-ds -n kube-system

echo "=== Step 5: Ensure CNI configs exist ==="
if [ ! -f /etc/cni/net.d/05-cilium.conflist ]; then
  echo "Writing missing 05-cilium.conflist..."
  sudo tee /etc/cni/net.d/05-cilium.conflist << 'CNIEOF'
{
  "cniVersion": "0.3.1",
  "name": "cilium",
  "plugins": [
    {
       "type": "cilium-cni",
       "enable-debug": false,
       "log-file": "/var/run/cilium/cilium-cni.log"
    }
  ]
}
CNIEOF
fi

if [ ! -f /etc/cni/net.d/00-multus.conf ]; then
  echo "Writing missing 00-multus.conf..."
  sudo tee /etc/cni/net.d/00-multus.conf << 'CNIEOF'
{
  "cniVersion": "0.3.1",
  "name": "multus-cni-network",
  "type": "multus-shim",
  "logLevel": "verbose",
  "logToStderr": true,
  "clusterNetwork": "/host/etc/cni/net.d/05-cilium.conflist"
}
CNIEOF
fi

echo "=== Step 6: Wait for Multus to be healthy ==="
kubectl rollout status ds/kube-multus-ds -n kube-system --timeout=120s
echo "Multus is healthy"

echo "=== Step 7: Start NFs one by one with Readiness Checks ==="
# Improved: Waits for each NF to actually be READY before starting the next
for nf in mongodb nrf udr udm ausf pcf nssf nef amf upf smf webui; do
  echo "Starting $nf..."
  kubectl scale deployment/$nf -n free5gc --replicas=1
  
  # Wait for the deployment to successfully roll out
  echo "Waiting for $nf to be Ready..."
  kubectl rollout status deployment/$nf -n free5gc --timeout=90s || echo "Warning: $nf timed out but continuing..."
done

echo "=== Final Status ==="
sleep 5
kubectl get pods -n free5gc