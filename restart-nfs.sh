#!/bin/bash
# Free5GC full cluster recovery script
# Handles post-preemption state reliably
# Works for: GCP spot VM restart, manual reboot, Multus CrashLoop
# Usage: bash restart-nfs.sh [--skip-monitoring]

set -o pipefail

SKIP_MONITORING=false
[[ "$1" == "--skip-monitoring" ]] && SKIP_MONITORING=true

# Colours
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()    { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $*"; }
error()   { echo -e "${RED}[ERROR]${NC} $*"; }

# ============================================================
# STEP 0 — Host-level CNI cleanup
# ============================================================
info "=== Step 0: Host-level CNI cleanup ==="
sudo ip -all netns delete 2>/dev/null || true
sudo rm -rf /var/run/multus/* 2>/dev/null || true
sudo rm -rf /var/lib/cni/networks/multus-cni-network/* 2>/dev/null || true
sudo rm -rf /var/lib/cni/networks/cbr0/* 2>/dev/null || true
info "CNI state cleared"

# ============================================================
# STEP 0.5 — Check and rebuild gtp5g if kernel was updated
# ============================================================
info "=== Step 0.5: gtp5g kernel module check ==="

GTP5G_SRC="${HOME}/gtp5g"
CURRENT_KERNEL=$(uname -r)
GTP5G_INSTALLED="/usr/lib/modules/${CURRENT_KERNEL}/kernel/drivers/net/gtp5g.ko"

if lsmod | grep -q gtp5g; then
  info "gtp5g already loaded for kernel ${CURRENT_KERNEL}"
elif [ -f "$GTP5G_INSTALLED" ]; then
  info "gtp5g built for ${CURRENT_KERNEL} — loading..."
  sudo modprobe gtp5g
  if lsmod | grep -q gtp5g; then
    info "gtp5g loaded successfully"
  else
    error "gtp5g failed to load — UPF will crash"
  fi
else
  warn "gtp5g not built for kernel ${CURRENT_KERNEL} — rebuilding from source..."
  if [ ! -d "$GTP5G_SRC" ]; then
    error "gtp5g source not found at ${GTP5G_SRC} — cannot rebuild"
    error "Run: git clone https://github.com/free5gc/gtp5g.git ~/gtp5g"
    exit 1
  fi

  cd "$GTP5G_SRC"
  make clean
  make -j$(nproc) CC=gcc-12 2>&1 | tail -5

  if [ $? -ne 0 ]; then
    error "gtp5g build failed — check gcc-12 is installed: sudo apt install gcc-12"
    exit 1
  fi

  sudo make install
  sudo modprobe gtp5g

  if lsmod | grep -q gtp5g; then
    info "gtp5g rebuilt and loaded for kernel ${CURRENT_KERNEL}"
  else
    error "gtp5g loaded failed after rebuild"
    exit 1
  fi

  cd - > /dev/null
fi

# ============================================================
# STEP 1 — Force delete all Unknown/stuck pods cluster-wide
# ============================================================
info "=== Step 1: Force delete Unknown/stuck pods cluster-wide ==="
for ns in free5gc kube-system monitoring; do
  STUCK=$(kubectl get pods -n $ns --no-headers 2>/dev/null | \
    grep -E "Unknown|Terminating|OOMKilled|Error|Completed|CrashLoopBackOff" | \
    awk '{print $1}')
  if [[ -n "$STUCK" ]]; then
    echo "$STUCK" | xargs -r kubectl delete pod -n $ns --force --grace-period=0 2>/dev/null
    info "Cleaned stuck pods in $ns"
  else
    info "No stuck pods in $ns"
  fi
done

# ============================================================
# STEP 2 — Scale down all free5gc NFs immediately
# ============================================================
info "=== Step 2: Scale down all free5gc NFs ==="
kubectl get deployments -n free5gc -o name | \
  xargs kubectl scale -n free5gc --replicas=0 2>/dev/null || true
sleep 3

# Force delete any ContainerCreating pods in free5gc
kubectl get pods -n free5gc --no-headers | \
  grep "ContainerCreating" | awk '{print $1}' | \
  xargs -r kubectl delete pod -n free5gc --force --grace-period=0 2>/dev/null || true

info "All free5gc NFs scaled down"

# ============================================================
# STEP 3 — Ensure CNI config files exist
# ============================================================
info "=== Step 3: Ensure CNI config files ==="
if [ ! -f /etc/cni/net.d/05-cilium.conflist ]; then
  warn "Writing missing 05-cilium.conflist..."
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
  warn "Writing missing 00-multus.conf..."
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
info "CNI configs verified"

# ============================================================
# STEP 4 — Restart Multus and wait for it to be healthy
# ============================================================
info "=== Step 4: Restart Multus ==="
kubectl rollout restart ds kube-multus-ds -n kube-system
sleep 5

info "Waiting for Multus daemonset to be ready..."
MULTUS_TIMEOUT=180
MULTUS_ELAPSED=0
until kubectl get ds kube-multus-ds -n kube-system -o jsonpath='{.status.numberReady}' 2>/dev/null | grep -q "^[1-9]"; do
  if [ $MULTUS_ELAPSED -ge $MULTUS_TIMEOUT ]; then
    error "Multus not ready after ${MULTUS_TIMEOUT}s — check: kubectl describe ds kube-multus-ds -n kube-system"
    exit 1
  fi
  echo -n "."
  sleep 5
  MULTUS_ELAPSED=$((MULTUS_ELAPSED + 5))
done
echo ""

# Extra wait — Multus pod Running doesn't mean socket is ready
info "Multus pod running — waiting 15s for socket to initialise..."
sleep 15

# Verify Multus socket exists
if [ ! -S /var/run/multus/multus.sock ] && [ ! -f /var/run/multus/multus.sock ]; then
  warn "Multus socket not found at /var/run/multus/multus.sock — waiting 15s more..."
  sleep 15
fi
info "Multus is healthy"

# ============================================================
# STEP 5 — Wait for CoreDNS to be ready
# ============================================================
info "=== Step 5: Wait for CoreDNS ==="

# Force delete stuck coredns pods
kubectl get pods -n kube-system -l k8s-app=kube-dns --no-headers | \
  grep -E "ContainerCreating|Unknown|Error" | awk '{print $1}' | \
  xargs -r kubectl delete pod -n kube-system --force --grace-period=0 2>/dev/null || true

COREDNS_TIMEOUT=120
COREDNS_ELAPSED=0
until kubectl get pods -n kube-system -l k8s-app=kube-dns --no-headers 2>/dev/null | grep -q "Running"; do
  if [ $COREDNS_ELAPSED -ge $COREDNS_TIMEOUT ]; then
    warn "CoreDNS not ready after ${COREDNS_TIMEOUT}s — continuing anyway"
    break
  fi
  echo -n "."
  sleep 5
  COREDNS_ELAPSED=$((COREDNS_ELAPSED + 5))
done
echo ""
info "CoreDNS ready"

# ============================================================
# STEP 6 — Restart monitoring stack
# ============================================================
if [ "$SKIP_MONITORING" = false ]; then
  info "=== Step 6: Restart monitoring stack ==="

  # Force delete stuck monitoring pods
  kubectl get pods -n monitoring --no-headers 2>/dev/null | \
    grep -E "Unknown|Terminating|Error|Completed|CrashLoopBackOff" | awk '{print $1}' | \
    xargs -r kubectl delete pod -n monitoring --force --grace-period=0 2>/dev/null || true

  # Restart deployments
  kubectl rollout restart deployment -n monitoring 2>/dev/null || true

  # Restart StatefulSets (Prometheus)
  kubectl rollout restart statefulset -n monitoring 2>/dev/null || true

  info "Monitoring stack restarted — pods will recover in background"
else
  info "=== Step 6: Skipping monitoring (--skip-monitoring) ==="
fi

# ============================================================
# STEP 7 — Restart kube-system components
# ============================================================
info "=== Step 7: Restart kube-system components ==="

# Restart local-path-provisioner and metrics-server if Completed/Error
for deploy in local-path-provisioner metrics-server; do
  STATUS=$(kubectl get deployment $deploy -n kube-system -o jsonpath='{.status.readyReplicas}' 2>/dev/null)
  if [[ -z "$STATUS" || "$STATUS" == "0" ]]; then
    warn "Restarting $deploy..."
    kubectl rollout restart deployment/$deploy -n kube-system 2>/dev/null || true
  fi
done

# Restart hubble if stuck
kubectl get pods -n kube-system -l app.kubernetes.io/name=hubble-ui --no-headers 2>/dev/null | \
  grep -E "Unknown|Error|Completed" | awk '{print $1}' | \
  xargs -r kubectl delete pod -n kube-system --force --grace-period=0 2>/dev/null || true

kubectl get pods -n kube-system -l app.kubernetes.io/name=hubble-relay --no-headers 2>/dev/null | \
  grep -E "Unknown|Error|Completed|CrashLoopBackOff" | awk '{print $1}' | \
  xargs -r kubectl delete pod -n kube-system --force --grace-period=0 2>/dev/null || true

info "kube-system components restarted"

# ============================================================
# STEP 8 — Start free5gc NFs in order
# ============================================================
info "=== Step 8: Start free5gc NFs in order ==="

NF_ORDER="mongodb nrf udr udm ausf pcf nssf nef amf upf smf webui"
NF_TIMEOUT=120

for nf in $NF_ORDER; do
  info "Starting $nf..."
  kubectl scale deployment/$nf -n free5gc --replicas=1 2>/dev/null || {
    warn "$nf deployment not found — skipping"
    continue
  }

  ELAPSED=0
  until kubectl get pods -n free5gc -l app=$nf --no-headers 2>/dev/null | grep -q "Running"; do
    if [ $ELAPSED -ge $NF_TIMEOUT ]; then
      warn "$nf not Running after ${NF_TIMEOUT}s — checking for sandbox errors..."
      # Check if it's a Multus EOF — if so, force delete and retry once
      POD=$(kubectl get pods -n free5gc -l app=$nf --no-headers | awk '{print $1}' | head -1)
      if [[ -n "$POD" ]]; then
        EVENTS=$(kubectl describe pod $POD -n free5gc 2>/dev/null | grep "EOF\|multus\|sandbox" | head -3)
        if [[ -n "$EVENTS" ]]; then
          warn "Multus EOF detected on $nf — force deleting and retrying..."
          kubectl delete pod $POD -n free5gc --force --grace-period=0 2>/dev/null
          sleep 10
          ELAPSED=0
          continue
        fi
      fi
      warn "$nf timed out — continuing to next NF"
      break
    fi
    echo -n "."
    sleep 5
    ELAPSED=$((ELAPSED + 5))
  done
  echo ""
  info "$nf is Running"
done

# ============================================================
# FINAL STATUS
# ============================================================
echo ""
info "=== Final Status ==="
echo ""
echo "--- free5gc ---"
kubectl get pods -n free5gc
echo ""
echo "--- monitoring ---"
kubectl get pods -n monitoring
echo ""
echo "--- kube-system (key pods) ---"
kubectl get pods -n kube-system | grep -E "multus|coredns|hubble|local-path|metrics-server"
echo ""

# Summary
RUNNING=$(kubectl get pods -n free5gc --no-headers | grep "Running" | wc -l)
TOTAL=$(kubectl get pods -n free5gc --no-headers | wc -l)
info "free5gc: ${RUNNING}/${TOTAL} pods Running"

GRAFANA=$(kubectl get pods -n monitoring -l app.kubernetes.io/name=grafana --no-headers 2>/dev/null | grep "Running" | wc -l)
PROM=$(kubectl get pods -n monitoring -l app.kubernetes.io/instance=kube-prom-stack --no-headers 2>/dev/null | grep "Running" | wc -l)
info "Grafana running: $GRAFANA | Prometheus stack running: $PROM"
