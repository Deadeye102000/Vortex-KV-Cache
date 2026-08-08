package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"vortex-cache/internal/resp"
)

type NodeMetric struct {
	Name       string  `json:"name"`
	Addr       string  `json:"addr"`
	Status     string  `json:"status"` // "UP" or "DOWN"
	Role       string  `json:"role"`   // "master" or "slave"
	Keys       int64   `json:"keys"`
	UptimeSec  int64   `json:"uptime_sec"`
	MasterHost string  `json:"master_host,omitempty"`
	LinkStatus string  `json:"link_status,omitempty"`
	LagMs      float64 `json:"lag_ms"`
	Offset     int64   `json:"offset"`
	Error      string  `json:"error,omitempty"`
}

type TopologyResponse struct {
	Timestamp string       `json:"timestamp"`
	Nodes     []NodeMetric `json:"nodes"`
}

func pollNode(name, addr string) NodeMetric {
	metric := NodeMetric{
		Name:   name,
		Addr:   addr,
		Status: "DOWN",
		Role:   "unknown",
	}

	conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
	if err != nil {
		metric.Error = err.Error()
		return metric
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(1 * time.Second))
	writer := resp.NewWriter(conn)
	reader := resp.NewReader(conn)

	if err := writer.WriteArray([]resp.Value{resp.NewBulkStringFromString("INFO")}); err != nil {
		metric.Error = err.Error()
		return metric
	}
	if err := writer.Flush(); err != nil {
		metric.Error = err.Error()
		return metric
	}

	res, err := reader.ReadValue()
	if err != nil {
		metric.Error = err.Error()
		return metric
	}

	metric.Status = "UP"
	infoStr := string(res.Bulk)
	if infoStr == "" {
		infoStr = res.Str
	}

	lines := strings.Split(infoStr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		k, v := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])

		switch k {
		case "role":
			metric.Role = v
		case "master_host":
			metric.MasterHost = v
		case "master_link_status":
			metric.LinkStatus = v
		case "uptime_in_seconds":
			metric.UptimeSec, _ = strconv.ParseInt(v, 10, 64)
		case "slave_repl_offset", "master_repl_offset":
			metric.Offset, _ = strconv.ParseInt(v, 10, 64)
		case "db0":
			if idx := strings.Index(v, "keys="); idx != -1 {
				sub := v[idx+5:]
				if commaIdx := strings.Index(sub, ","); commaIdx != -1 {
					sub = sub[:commaIdx]
				}
				metric.Keys, _ = strconv.ParseInt(sub, 10, 64)
			}
		}
	}

	return metric
}

func main() {
	port := flag.Int("port", 8080, "Dashboard HTTP port")
	nodesFlag := flag.String("nodes", "127.0.0.1:6379,127.0.0.1:6380,127.0.0.1:6381", "Comma-separated list of target cluster node addresses (name=addr or addr)")
	flag.Parse()

	nodeTargets := parseNodeTargets(*nodesFlag)

	http.HandleFunc("/api/topology", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		var wg sync.WaitGroup
		results := make([]NodeMetric, len(nodeTargets))

		for i, target := range nodeTargets {
			wg.Add(1)
			go func(idx int, t nodeTarget) {
				defer wg.Done()
				results[idx] = pollNode(t.name, t.addr)
			}(i, target)
		}
		wg.Wait()

		respData := TopologyResponse{
			Timestamp: time.Now().Format(time.RFC3339),
			Nodes:     results,
		}
		_ = json.NewEncoder(w).Encode(respData)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(dashboardHTML))
	})

	addr := fmt.Sprintf(":%d", *port)
	fmt.Printf("Vortex Cache Topology Dashboard listening on http://localhost%s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}

type nodeTarget struct {
	name string
	addr string
}

func parseNodeTargets(raw string) []nodeTarget {
	parts := strings.Split(raw, ",")
	var targets []nodeTarget
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		if strings.Contains(p, "=") {
			kv := strings.SplitN(p, "=", 2)
			targets = append(targets, nodeTarget{name: kv[0], addr: kv[1]})
		} else {
			name := fmt.Sprintf("node-%d", i+1)
			if i == 0 {
				name = "vortex-master"
			} else {
				name = fmt.Sprintf("vortex-replica-%d", i)
			}
			targets = append(targets, nodeTarget{name: name, addr: p})
		}
	}
	return targets
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Vortex Cache — Live Cluster Topology</title>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Geist+Mono:wght@400;500;600;700&family=Inter:wght@300;400;500;600;700&display=swap" rel="stylesheet">
  <style>
    :root {
      --bg-dark: #090d16;
      --card-bg: rgba(255, 255, 255, 0.03);
      --card-border: rgba(255, 255, 255, 0.08);
      --gold: #f3c677;
      --gold-glow: rgba(243, 198, 119, 0.15);
      --cyan: #00f2fe;
      --green: #10b981;
      --red: #f43f5e;
      --text-main: #f3f4f6;
      --text-muted: #9ca3af;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      background-color: var(--bg-dark); color: var(--text-main);
      font-family: 'Inter', sans-serif; min-height: 100vh; padding: 2rem;
    }
    .container { max-width: 1200px; margin: 0 auto; }
    header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 2rem; padding-bottom: 1.5rem; border-bottom: 1px solid var(--card-border); }
    .logo-section { display: flex; align-items: center; gap: 1rem; }
    .logo-icon {
      width: 44px; height: 44px; background: linear-gradient(135deg, var(--gold), #d97706);
      border-radius: 12px; display: flex; align-items: center; justify-content: center;
      font-family: 'Geist Mono', monospace; font-weight: 700; color: #000; font-size: 1.4rem;
    }
    h1 { font-size: 1.5rem; font-weight: 700; }
    .subtitle { font-size: 0.875rem; color: var(--text-muted); }
    .live-badge {
      display: flex; align-items: center; gap: 0.5rem; background: rgba(16, 185, 129, 0.1);
      border: 1px solid rgba(16, 185, 129, 0.3); padding: 0.4rem 0.8rem; border-radius: 9999px;
      font-family: 'Geist Mono', monospace; font-size: 0.75rem; color: var(--green);
    }
    .metrics-bar { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 1rem; margin-bottom: 2rem; }
    .metric-card { background: var(--card-bg); border: 1px solid var(--card-border); border-radius: 12px; padding: 1.25rem; }
    .metric-label { font-size: 0.75rem; text-transform: uppercase; color: var(--text-muted); font-family: 'Geist Mono', monospace; }
    .metric-value { font-size: 1.75rem; font-weight: 700; margin-top: 0.25rem; font-family: 'Geist Mono', monospace; color: var(--gold); }
    .nodes-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(320px, 1fr)); gap: 1.5rem; }
    .node-card { background: var(--card-bg); border: 1px solid var(--card-border); border-radius: 16px; padding: 1.5rem; }
    .node-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; }
    .node-title { font-weight: 600; font-size: 1.1rem; }
    .node-addr { font-family: 'Geist Mono', monospace; font-size: 0.8rem; color: var(--text-muted); }
    .role-pill { padding: 0.25rem 0.6rem; border-radius: 6px; font-family: 'Geist Mono', monospace; font-size: 0.7rem; font-weight: 600; text-transform: uppercase; }
    .role-master { background: rgba(243, 198, 119, 0.15); color: var(--gold); border: 1px solid rgba(243, 198, 119, 0.4); }
    .role-slave { background: rgba(0, 242, 254, 0.15); color: var(--cyan); border: 1px solid rgba(0, 242, 254, 0.4); }
    .role-offline { background: rgba(244, 63, 94, 0.15); color: var(--red); border: 1px solid rgba(244, 63, 94, 0.4); }
    .stat-row { display: flex; justify-content: space-between; padding: 0.5rem 0; border-bottom: 1px dashed rgba(255, 255, 255, 0.05); font-size: 0.875rem; }
    .stat-label { color: var(--text-muted); }
    .stat-val { font-family: 'Geist Mono', monospace; }
  </style>
</head>
<body>
  <div class="container">
    <header>
      <div class="logo-section">
        <div class="logo-icon">V</div>
        <div>
          <h1>Vortex Cache Engine</h1>
          <div class="subtitle">Live Master-Replica Topology & Replication Monitor</div>
        </div>
      </div>
      <div class="live-badge"><span>● LIVE RESP STREAM</span></div>
    </header>
    <div class="metrics-bar">
      <div class="metric-card"><div class="metric-label">Active Nodes</div><div class="metric-value" id="total-nodes">0 / 0</div></div>
      <div class="metric-card"><div class="metric-label">Total Cluster Keys</div><div class="metric-value" id="total-keys">0</div></div>
      <div class="metric-card"><div class="metric-label">Master Address</div><div class="metric-value" id="master-addr" style="font-size: 1.2rem; color: var(--text-main);">—</div></div>
    </div>
    <div class="nodes-grid" id="nodes-grid"></div>
  </div>
  <script>
    async function fetchTopology() {
      try {
        const res = await fetch('/api/topology');
        const data = await res.json();
        render(data);
      } catch (e) {}
    }
    function render(data) {
      const grid = document.getElementById('nodes-grid');
      grid.innerHTML = '';
      let totalKeys = 0, active = 0, master = '—';
      data.nodes.forEach(n => {
        if (n.status === 'UP') active++;
        totalKeys += n.keys || 0;
        if (n.role === 'master' && n.status === 'UP') master = n.addr;
        const card = document.createElement('div');
        card.className = 'node-card';
        const roleClass = n.status === 'DOWN' ? 'role-offline' : (n.role === 'master' ? 'role-master' : 'role-slave');
        const roleText = n.status === 'DOWN' ? 'OFFLINE' : n.role.toUpperCase();
        card.innerHTML = '<div class="node-header"><div><div class="node-title">' + n.name + '</div><div class="node-addr">' + n.addr + '</div></div><span class="role-pill ' + roleClass + '">' + roleText + '</span></div>' +
          '<div class="stat-row"><span class="stat-label">Status</span><span class="stat-val">' + n.status + '</span></div>' +
          '<div class="stat-row"><span class="stat-label">Keys</span><span class="stat-val">' + (n.keys || 0) + '</span></div>' +
          '<div class="stat-row"><span class="stat-label">Uptime</span><span class="stat-val">' + (n.uptime_sec || 0) + 's</span></div>';
        grid.appendChild(card);
      });
      document.getElementById('total-nodes').innerText = active + ' / ' + data.nodes.length;
      document.getElementById('total-keys').innerText = totalKeys;
      document.getElementById('master-addr').innerText = master;
    }
    fetchTopology();
    setInterval(fetchTopology, 1000);
  </script>
</body>
</html>`
