package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Info holds cluster and BBR deployment information.
type Info struct {
	Cluster    ClusterInfo `json:"cluster"`
	BBR        BBRInfo     `json:"bbr"`
	PortForward PortForwardInfo `json:"portForward"`
	Error      string      `json:"error,omitempty"`
}

type ClusterInfo struct {
	Server    string `json:"server"`
	User      string `json:"user"`
	Namespace string `json:"namespace"`
	Connected bool   `json:"connected"`
}

type BBRInfo struct {
	PodName   string   `json:"podName"`
	Status    string   `json:"status"`
	Image     string   `json:"image"`
	Plugins   []Plugin `json:"plugins"`
	Ports     []Port   `json:"ports"`
	Uptime    string   `json:"uptime"`
	Namespace string   `json:"namespace"`
}

type Plugin struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Config string `json:"config,omitempty"`
}

type Port struct {
	Name string `json:"name"`
	Port int    `json:"port"`
}

type PortForwardInfo struct {
	Command   string `json:"command"`
	GRPCPort  int    `json:"grpcPort"`
	Target    string `json:"target"`
}

// Login logs into an OpenShift cluster via oc login.
func Login(ctx context.Context, server, username, password string) error {
	args := []string{"login", server, "--username=" + username, "--password=" + password, "--insecure-skip-tls-verify"}
	_, err := runOC(ctx, args...)
	return err
}

// Logout logs out of the current cluster.
func Logout(ctx context.Context) error {
	_, err := runOC(ctx, "logout")
	return err
}

// GetInfo queries the cluster for BBR deployment information.
func GetInfo(ctx context.Context, namespace string) *Info {
	info := &Info{}

	// Cluster connectivity
	info.Cluster = getClusterInfo(ctx)
	if !info.Cluster.Connected {
		info.Error = "not connected to cluster"
		return info
	}

	if namespace == "" {
		namespace = "bbr-plugins"
	}

	// BBR pod info
	info.BBR = getBBRInfo(ctx, namespace)

	// Port forward command
	info.PortForward = PortForwardInfo{
		Command:  fmt.Sprintf("oc port-forward svc/bbr-plugins 9004:9004 -n %s", namespace),
		GRPCPort: 9004,
		Target:   "localhost:9004",
	}

	return info
}

func getClusterInfo(ctx context.Context) ClusterInfo {
	ci := ClusterInfo{}

	server, err := runOC(ctx, "whoami", "--show-server")
	if err != nil {
		return ci
	}
	ci.Server = strings.TrimSpace(server)

	user, err := runOC(ctx, "whoami")
	if err != nil {
		return ci
	}
	ci.User = strings.TrimSpace(user)
	ci.Connected = true

	ns, _ := runOC(ctx, "project", "-q")
	ci.Namespace = strings.TrimSpace(ns)

	return ci
}

func getBBRInfo(ctx context.Context, namespace string) BBRInfo {
	bi := BBRInfo{Namespace: namespace}

	// Get pod info as JSON
	podsJSON, err := runOC(ctx, "get", "pods", "-n", namespace, "-l", "app=bbr-plugins",
		"-o", "jsonpath={.items[?(@.status.phase=='Running')]}")
	if err != nil || podsJSON == "" {
		bi.Status = "not deployed"
		return bi
	}

	// Parse pod JSON for basic info
	var pod struct {
		Metadata struct {
			Name              string    `json:"name"`
			CreationTimestamp  time.Time `json:"creationTimestamp"`
		} `json:"metadata"`
		Spec struct {
			Containers []struct {
				Name  string `json:"name"`
				Image string `json:"image"`
				Args  []string `json:"args"`
				Ports []struct {
					Name          string `json:"name"`
					ContainerPort int    `json:"containerPort"`
				} `json:"ports"`
			} `json:"containers"`
		} `json:"spec"`
		Status struct {
			Phase string `json:"phase"`
		} `json:"status"`
	}

	if err := json.Unmarshal([]byte(podsJSON), &pod); err != nil {
		bi.Status = "error parsing pod info"
		return bi
	}

	bi.PodName = pod.Metadata.Name
	bi.Status = pod.Status.Phase
	bi.Uptime = time.Since(pod.Metadata.CreationTimestamp).Round(time.Second).String()

	if len(pod.Spec.Containers) > 0 {
		c := pod.Spec.Containers[0]
		bi.Image = c.Image

		for _, p := range c.Ports {
			bi.Ports = append(bi.Ports, Port{Name: p.Name, Port: p.ContainerPort})
		}

		// Parse --plugin args
		bi.Plugins = parsePluginArgs(c.Args)
	}

	return bi
}

func parsePluginArgs(args []string) []Plugin {
	var plugins []Plugin
	for i, arg := range args {
		if arg == "--plugin" && i+1 < len(args) {
			p := parsePluginSpec(args[i+1])
			if p.Type != "" {
				plugins = append(plugins, p)
			}
		}
		// Also handle --plugin=value format
		if strings.HasPrefix(arg, "--plugin=") {
			val := strings.TrimPrefix(arg, "--plugin=")
			p := parsePluginSpec(val)
			if p.Type != "" {
				plugins = append(plugins, p)
			}
		}
	}
	return plugins
}

func parsePluginSpec(spec string) Plugin {
	// Format: type:name[:json_config]
	parts := strings.SplitN(spec, ":", 3)
	p := Plugin{}
	if len(parts) >= 1 {
		p.Type = parts[0]
	}
	if len(parts) >= 2 {
		p.Name = parts[1]
	}
	if len(parts) >= 3 {
		p.Config = parts[2]
	}
	return p
}

func runOC(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "oc", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("oc %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}
