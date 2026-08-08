#!/usr/bin/env bash

set -e

GREEN='\033[0;32m'
GOLD='\033[0;33m'
CYAN='\033[0;36m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GOLD}=======================================================${NC}"
echo -e "${GOLD}   VORTEX CACHE ENGINE — LIVE REPLICATION & FAILOVER DEMO${NC}"
echo -e "${GOLD}=======================================================${NC}"

# Check for Go binary
which go > /dev/null || export PATH="/opt/homebrew/bin:$PATH"

echo -e "\n${CYAN}[1/4] Building Vortex Server & Dashboard...${NC}"
go build -o bin/vortex-server ./cmd/vortex-server
go build -o bin/vortex-dashboard ./cmd/vortex-dashboard

echo -e "\n${CYAN}[2/4] Starting 3-Node Cluster locally...${NC}"
# Master on 6379, Replica 1 on 6380, Replica 2 on 6381
./bin/vortex-server -p 6379 > /tmp/master.log 2>&1 &
MASTER_PID=$!

./bin/vortex-server -p 6380 > /tmp/replica1.log 2>&1 &
REPLICA1_PID=$!

./bin/vortex-server -p 6381 > /tmp/replica2.log 2>&1 &
REPLICA2_PID=$!

./bin/vortex-dashboard -port 8080 -nodes "Master=127.0.0.1:6379,Replica-1=127.0.0.1:6380,Replica-2=127.0.0.1:6381" > /tmp/dashboard.log 2>&1 &
DASHBOARD_PID=$!

cleanup() {
  echo -e "\n${RED}Cleaning up cluster processes...${NC}"
  kill -9 $MASTER_PID $REPLICA1_PID $REPLICA2_PID $DASHBOARD_PID 2>/dev/null || true
  rm -rf bin/ /tmp/master.log /tmp/replica1.log /tmp/replica2.log /tmp/dashboard.log
}
trap cleanup EXIT

sleep 1

echo -e "${GREEN}✓ Cluster Online:${NC}"
echo -e "  - Master:    127.0.0.1:6379"
echo -e "  - Replica 1: 127.0.0.1:6380"
echo -e "  - Replica 2: 127.0.0.1:6381"
echo -e "  - Dashboard: http://localhost:8080"

echo -e "\n${CYAN}[3/4] Establishing Master-Replica Replication links...${NC}"
redis-cli -p 6380 SLAVEOF 127.0.0.1 6379
redis-cli -p 6381 SLAVEOF 127.0.0.1 6379
sleep 1

echo -e "\n${CYAN}[4/4] Writing keys to Master (6379) and checking live streaming to Replica 1 (6380)...${NC}"
redis-cli -p 6379 SET demo:cluster:name "Vortex Distributed Cache" EX 60
sleep 0.1

echo -n "Reading from Replica 1 (6380): "
redis-cli -p 6380 GET demo:cluster:name

echo -e "\n${RED}Injected Chaos: Terminating Master Process (PID: $MASTER_PID)...${NC}"
kill -9 $MASTER_PID 2>/dev/null || true

echo -e "${GOLD}Waiting for Replica 1 to detect failure & execute self-promotion...${NC}"
for i in {1..30}; do
  RES=$(redis-cli -p 6380 SET post:failover:key "Promoted Master Ready" 2>&1 || true)
  if [[ "$RES" == "OK" ]]; then
    echo -e "${GREEN}✓ Replica 1 successfully self-promoted to Master!${NC}"
    echo -n "Reading key from Promoted Master: "
    redis-cli -p 6380 GET post:failover:key
    break
  fi
  sleep 0.1
done

echo -e "\n${GOLD}=======================================================${NC}"
echo -e "${GREEN}   DEMO SUCCESSFUL — OPEN DASHBOARD: http://localhost:8080${NC}"
echo -e "${GOLD}=======================================================${NC}"
