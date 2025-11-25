package main

import (
	"fmt"
)

// modulesCmd calls GET /modules via SDK and prints names
func modulesCmd() {
	names, err := s.Client.GetModules()
	if err != nil {
		fmt.Printf("GetModules error: %v\n", err)
		return
	}
	if len(names) == 0 {
		fmt.Println("No modules found (empty list)")
		return
	}
	fmt.Println("Modules:")
	for i, name := range names {
		fmt.Printf("  %d. %s\n", i+1, name)
	}
}

// trySendCmd calls POST /trysend via SDK with pid and target
func trySendCmd(pid, target string) {
	if err := s.Client.TrySend(pid, target); err != nil {
		fmt.Printf("TrySend error: %v\n", err)
		return
	}
	fmt.Println("TrySend OK")
}

// getCacheCmd calls GET /cache/:pid/:key via SDK and prints the value
func getCacheCmd(pid, key string) {
	val, err := s.Client.GetCache(pid, key)
	if err != nil {
		fmt.Printf("GetCache error: %v\n", err)
		return
	}
	fmt.Printf("Cache[%s/%s] = %q\n", pid, key, val)
}

// nodesCmd calls GET /nodes and prints the registry map
func nodesCmd() {
	m, err := s.Client.GetNodes()
	if err != nil {
		fmt.Printf("GetNodes error: %v\n", err)
		return
	}
	if len(m) == 0 {
		fmt.Println("No nodes found")
		return
	}
	fmt.Println("Nodes:")
	i := 1
	for accid, node := range m {
		fmt.Printf("  %d. %s (%s) role=%s url=%s\n", i, node.Name, accid, node.Role, node.URL)
		i++
	}
}

// nodeCmd calls GET /node/:accid and prints the node
func nodeCmd(accid string) {
	node, err := s.Client.GetNode(accid)
	if err != nil {
		fmt.Printf("GetNode error: %v\n", err)
		return
	}
	if node == nil {
		fmt.Println("Node not found")
		return
	}
	fmt.Printf("Node %s: name=%s role=%s url=%s desc=%s\n", accid, node.Name, node.Role, node.URL, node.Desc)
}

// nodesByProcessCmd calls GET /nodesByProcess/:pid and prints the nodes
func nodesByProcessCmd(pid string) {
	nodes, err := s.Client.GetNodesByProcess(pid)
	if err != nil {
		fmt.Printf("GetNodesByProcess error: %v\n", err)
		return
	}
	if len(nodes) == 0 {
		fmt.Println("No nodes found for process")
		return
	}
	fmt.Printf("Nodes for process %s:\n", pid)
	for i, node := range nodes {
		fmt.Printf("  %d. %s (%s) role=%s url=%s\n", i+1, node.Name, node.AccId, node.Role, node.URL)
	}
}

// processesCmd calls GET /processes/:accid and prints the process list
func processesCmd(accid string) {
	pids, err := s.Client.GetProcesses(accid)
	if err != nil {
		fmt.Printf("GetProcesses error: %v\n", err)
		return
	}
	if len(pids) == 0 {
		fmt.Println("No processes found for account")
		return
	}
	fmt.Printf("Processes for %s:\n", accid)
	for i, pid := range pids {
		fmt.Printf("  %d. %s\n", i+1, pid)
	}
}
