#!/bin/bash
# Free5GC NF restart in correct order after VM boot

echo "=== Step 1: Clean up stuck pods ==="
kubectl get pods -n free5gc | grep -E "Unknown|Terminating" | awk '{print $1}' | \
  xargs -r kubectl delete pod -n free5gc --force --grace-period=0 2>/dev/null

echo "=== Step 2: Clean up stale ReplicaSets ==="
kubectl get rs -n free5gc -o name | while read rs; do
  desired=$(kubectl get $rs -n free5gc -o jsonpath='{.spec.replicas}')
  if [ "$desired" = "0" ]; then
    echo "Deleting stale RS: $rs"
    kubectl delete $rs -n free5gc
  fi
done
sleep 5

echo "=== Step 3: Wait for Multus to be healthy ==="
until kubectl get pods -n kube-system | grep multus | grep -q "1/1.*Running"; do
  echo "Waiting for Multus..."
  sleep 5
done
echo "Multus is healthy"

echo "=== Step 4: Scale down all NFs ==="
kubectl get deployments -n free5gc -o name | \
  xargs -r kubectl scale -n free5gc --replicas=0 2>/dev/null || true
sleep 10

echo "=== Step 5: Delete upfgtp interface if it exists ==="
sudo ip link delete upfgtp 2>/dev/null && echo "Deleted upfgtp" || echo "upfgtp not present"

echo "=== Step 6: Start NFs one by one ==="
for nf in mongodb nrf udr udm ausf pcf nssf nef amf upf smf webui; do
  echo "Starting $nf..."
  kubectl scale deployment/$nf -n free5gc --replicas=1
  sleep 8
done

echo "=== Step 7: Wait for pods ==="
sleep 30
kubectl get pods -n free5gc
