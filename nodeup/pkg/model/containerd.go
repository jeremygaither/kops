/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package model

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/pelletier/go-toml"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
	"k8s.io/kops/nodeup/pkg/model/resources"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/flagbuilder"
	"k8s.io/kops/pkg/systemd"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/nodeup/nodetasks"
	"k8s.io/kops/util/pkg/distributions"
)

const containerdConfigFilePath = "/etc/containerd/config.toml"

// ContainerdBuilder install containerd (just the packages at the moment)
type ContainerdBuilder struct {
	*NodeupModelContext
}

var _ fi.NodeupModelBuilder = &ContainerdBuilder{}

// Build is responsible for configuring the containerd daemon
func (b *ContainerdBuilder) Build(c *fi.NodeupModelBuilderContext) error {
	if b.skipInstall() {
		klog.Infof("SkipInstall is set to true; won't install containerd")
		return nil
	}

	installContainerd := true

	// @check: neither flatcar nor containeros need provision containerd.service, just the containerd daemon options
	switch b.Distribution {
	case distributions.DistributionFlatcar:
		klog.Infof("Detected Flatcar; won't install containerd")
		installContainerd = false
		b.buildSystemdServiceOverrideFlatcar(c)
	case distributions.DistributionContainerOS:
		klog.Infof("Detected ContainerOS; won't install containerd")
		installContainerd = false
		b.buildSystemdServiceOverrideContainerOS(c)
	}

	// Using containerd with Kubenet requires special configuration.
	// This is a temporary backwards-compatible solution for kubenet users and will be deprecated when Kubenet is deprecated:
	// https://github.com/containerd/containerd/blob/master/docs/cri/config.md#cni-config-template
	if b.NodeupConfig.UsesKubenet {
		if err := b.buildCNIConfigTemplateFile(c); err != nil {
			return err
		}
		if err := b.buildIPMasqueradeRules(c); err != nil {
			return err
		}
	}

	// If there are containerd configuration overrides, apply them
	if err := b.buildConfigFile(c); err != nil {
		return err
	}

	if installContainerd {
		if err := b.installContainerd(c); err != nil {
			return err
		}
	}

	return nil
}

// installContainerd installs the binaries and services to run containerd.
// We break it out because on immutable OSes we only configure containerd, we don't install it.
func (b *ContainerdBuilder) installContainerd(c *fi.NodeupModelBuilderContext) error {
	// Add Apache2 license
	{
		t := &nodetasks.File{
			Path:     "/usr/share/doc/containerd/apache.txt",
			Contents: fi.NewStringResource(resources.ContainerdApache2License),
			Type:     nodetasks.FileType_File,
		}
		c.AddTask(t)
	}

	// Add containerd binaries from containerd release package
	f := b.Assets.FindMatches(regexp.MustCompile(`^bin/(containerd|ctr)`))
	if len(f) == 0 {
		// Add containerd binaries from containerd bundle package
		f = b.Assets.FindMatches(regexp.MustCompile(`^(\./)?usr/local/bin/(containerd|crictl|ctr)`))
	}
	if len(f) == 0 {
		// Add containerd binaries from Docker package (for ARM64 builds < v1.6.0)
		// https://github.com/containerd/containerd/pull/6196
		f = b.Assets.FindMatches(regexp.MustCompile(`^docker/(containerd|ctr)`))
	}
	if len(f) == 0 {
		return fmt.Errorf("unable to find any containerd binaries in assets")
	}
	for k, v := range f {
		fileTask := &nodetasks.File{
			Path:     filepath.Join("/usr/bin", k),
			Contents: v,
			Type:     nodetasks.FileType_File,
			Mode:     fi.PtrTo("0755"),
		}
		c.AddTask(fileTask)
	}

	// Add runc binary from https://github.com/opencontainers/runc
	// https://github.com/containerd/containerd/issues/6541
	f = b.Assets.FindMatches(regexp.MustCompile(`/runc\.(amd64|arm64)$`))
	if len(f) == 0 {
		// Add runc binary from containerd package (for builds < v1.6.0)
		f = b.Assets.FindMatches(regexp.MustCompile(`^(\./)?usr/local/sbin/runc$`))
	}
	if len(f) == 0 {
		// Add runc binary from Docker package (for ARM64 builds < v1.6.0)
		// https://github.com/containerd/containerd/pull/6196
		f = b.Assets.FindMatches(regexp.MustCompile(`^docker/runc$`))
	}
	if len(f) != 1 {
		return fmt.Errorf("error finding runc asset")
	}
	for _, v := range f {
		fileTask := &nodetasks.File{
			Path:     "/usr/sbin/runc",
			Contents: v,
			Type:     nodetasks.FileType_File,
			Mode:     fi.PtrTo("0755"),
		}
		c.AddTask(fileTask)
	}

	// Add configuration file for easier use of crictl
	b.addCrictlConfig(c)

	var containerdVersion string
	if b.NodeupConfig.ContainerdConfig != nil {
		containerdVersion = fi.ValueOf(b.NodeupConfig.ContainerdConfig.Version)
	} else {
		return fmt.Errorf("error finding contained version")
	}
	sv, err := semver.ParseTolerant(containerdVersion)
	if err != nil {
		return fmt.Errorf("error parsing container runtime version %q: %v", containerdVersion, err)
	}
	c.AddTask(b.buildSystemdService(sv))

	if err := b.buildSysconfigFile(c); err != nil {
		return err
	}

	return nil
}

func (b *ContainerdBuilder) buildSystemdService(containerdVersion semver.Version) *nodetasks.Service {
	// Based on https://github.com/containerd/containerd/blob/master/containerd.service

	manifest := &systemd.Manifest{}
	manifest.Set("Unit", "Description", "containerd container runtime")
	manifest.Set("Unit", "Documentation", "https://containerd.io")
	manifest.Set("Unit", "After", "network.target local-fs.target")

	manifest.Set("Service", "EnvironmentFile", "/etc/sysconfig/containerd")
	manifest.Set("Service", "EnvironmentFile", "/etc/environment")
	manifest.Set("Service", "ExecStartPre", "-/sbin/modprobe overlay")
	manifest.Set("Service", "ExecStart", "/usr/bin/containerd -c "+containerdConfigFilePath+" \"$CONTAINERD_OPTS\"")

	// notify the daemon's readiness to systemd
	manifest.Set("Service", "Type", "notify")

	// set delegate yes so that systemd does not reset the cgroups of containerd containers
	manifest.Set("Service", "Delegate", "yes")
	// kill only the containerd process, not all processes in the cgroup
	manifest.Set("Service", "KillMode", "process")

	manifest.Set("Service", "Restart", "always")
	manifest.Set("Service", "RestartSec", "5")

	manifest.Set("Service", "LimitNPROC", "infinity")
	manifest.Set("Service", "LimitCORE", "infinity")
	manifest.Set("Service", "LimitNOFILE", "1048576")
	manifest.Set("Service", "TasksMax", "infinity")

	// make killing of processes of this unit under memory pressure very unlikely
	manifest.Set("Service", "OOMScoreAdjust", "-999")

	manifest.Set("Install", "WantedBy", "multi-user.target")

	if b.NodeupConfig.KubeletConfig.CgroupDriver == "systemd" {
		cgroup := b.NodeupConfig.KubeletConfig.RuntimeCgroups
		if cgroup != "" {
			manifest.Set("Service", "Slice", strings.Trim(cgroup, "/")+".slice")
		}
	}

	manifestString := manifest.Render()
	klog.V(8).Infof("Built service manifest %q\n%s", "containerd", manifestString)

	service := &nodetasks.Service{
		Name:       "containerd.service",
		Definition: s(manifestString),
	}

	service.InitDefaults()

	return service
}

// buildSystemdServiceOverrideContainerOS is responsible for overriding the containerd service for ContainerOS
func (b *ContainerdBuilder) buildSystemdServiceOverrideContainerOS(c *fi.NodeupModelBuilderContext) {
	lines := []string{
		"[Service]",
		"EnvironmentFile=/etc/environment",
		"TasksMax=infinity",
	}
	contents := strings.Join(lines, "\n")

	c.AddTask(&nodetasks.File{
		Path:       "/etc/systemd/system/containerd.service.d/10-kops.conf",
		Contents:   fi.NewStringResource(contents),
		Type:       nodetasks.FileType_File,
		AfterFiles: []string{containerdConfigFilePath},
		OnChangeExecute: [][]string{
			{"systemctl", "daemon-reload"},
			{"systemctl", "restart", "containerd.service"},
			// We need to restart kops-configuration service since nodeup needs to load images
			// into containerd with the new config. We restart in the background because
			// kops-configuration is of type "one-shot", so the restart command will wait for
			// nodeup to finish executing.
			{"systemctl", "restart", "kops-configuration.service", "&"},
		},
	})
}

// buildSystemdServiceOverrideFlatcar is responsible for overriding the containerd service for Flatcar
func (b *ContainerdBuilder) buildSystemdServiceOverrideFlatcar(c *fi.NodeupModelBuilderContext) {
	lines := []string{
		"[Service]",
		"EnvironmentFile=/etc/environment",
		"ExecStart=",
		"ExecStart=/usr/bin/containerd --config " + containerdConfigFilePath,
	}
	contents := strings.Join(lines, "\n")

	c.AddTask(&nodetasks.File{
		Path:       "/etc/systemd/system/containerd.service.d/10-kops.conf",
		Contents:   fi.NewStringResource(contents),
		Type:       nodetasks.FileType_File,
		AfterFiles: []string{containerdConfigFilePath},
		OnChangeExecute: [][]string{
			{"systemctl", "daemon-reload"},
			{"systemctl", "restart", "containerd.service"},
			// We need to restart kops-configuration service since nodeup needs to load images
			// into containerd with the new config. We restart in the background because
			// kops-configuration is of type "one-shot", so the restart command will wait for
			// nodeup to finish executing.
			{"systemctl", "restart", "kops-configuration.service", "&"},
		},
	})
}

// buildSysconfigFile is responsible for creating the containerd sysconfig file
func (b *ContainerdBuilder) buildSysconfigFile(c *fi.NodeupModelBuilderContext) error {
	var containerd kops.ContainerdConfig
	if b.NodeupConfig.ContainerdConfig != nil {
		containerd = *b.NodeupConfig.ContainerdConfig
	}

	flagsString, err := flagbuilder.BuildFlags(&containerd)
	if err != nil {
		return fmt.Errorf("error building containerd flags: %v", err)
	}

	lines := []string{
		"CONTAINERD_OPTS=" + flagsString,
	}
	contents := strings.Join(lines, "\n")

	c.AddTask(&nodetasks.File{
		Path:     "/etc/sysconfig/containerd",
		Contents: fi.NewStringResource(contents),
		Type:     nodetasks.FileType_File,
	})

	return nil
}

// buildConfigFile is responsible for creating the containerd configuration file
func (b *ContainerdBuilder) buildConfigFile(c *fi.NodeupModelBuilderContext) error {
	var config string

	if b.NodeupConfig.ContainerdConfig != nil && b.NodeupConfig.ContainerdConfig.ConfigOverride != nil {
		config = fi.ValueOf(b.NodeupConfig.ContainerdConfig.ConfigOverride)
	} else {
		if cc, err := b.buildContainerdConfig(); err != nil {
			return err
		} else {
			config = cc
		}
	}
	c.AddTask(&nodetasks.File{
		Path:     containerdConfigFilePath,
		Contents: fi.NewStringResource(config),
		Type:     nodetasks.FileType_File,
	})
	return nil
}

// skipInstall determines if kops should skip the installation and configuration of containerd
func (b *ContainerdBuilder) skipInstall() bool {
	d := b.NodeupConfig.ContainerdConfig

	// don't skip install if the user hasn't specified anything
	if d == nil {
		return false
	}

	return d.SkipInstall
}

// addCrictlConfig creates /etc/crictl.yaml, which lets crictl work out-of-the-box.
func (b *ContainerdBuilder) addCrictlConfig(c *fi.NodeupModelBuilderContext) {
	conf := `
runtime-endpoint: unix:///run/containerd/containerd.sock
`

	c.AddTask(&nodetasks.File{
		Path:     "/etc/crictl.yaml",
		Contents: fi.NewStringResource(conf),
		Type:     nodetasks.FileType_File,
	})
}

// buildIPMasqueradeRules creates the DNAT rules.
// Network modes where pods don't have "real network" IPs, use NAT so that they assume the IP of the node.
func (b *ContainerdBuilder) buildIPMasqueradeRules(c *fi.NodeupModelBuilderContext) error {
	// TODO: Should we just rely on running nodeup on every boot, instead of setting up a systemd unit?

	if b.NodeupConfig.Networking.NonMasqueradeCIDR == "" {
		// We could fall back to the pod CIDR, that is likely a good universal
		klog.Infof("not setting up masquerade, as NonMasqueradeCIDR is not set")
		return nil
	}

	if strings.HasSuffix(b.NodeupConfig.Networking.NonMasqueradeCIDR, "/0") {
		klog.Infof("not setting up masquerade, as NonMasqueradeCIDR is %s", b.NodeupConfig.Networking.NonMasqueradeCIDR)
		return nil
	}

	// This is based on rules from gce/cos/configure-helper.sh and the old logic in kubenet_linux.go

	// We stick closer to the logic in kubenet_linux, both for compatibility, and because the GCE logic
	// skips masquerading for all private CIDR ranges, but this depends on an assumption that is likely GCE-specific.
	// On GCE custom routes are at the network level, on AWS they are at the route-table / subnet level.
	// We cannot generally assume that because something is in the private network space, that it can reach us.
	// If we adopt "native" pod IPs (GCP ip-alias, AWS VPC CNI, etc) we can likely move to rules closer to the upstream ones.
	script := `#!/bin/bash
# Built by kOps - do not edit

iptables -w -t nat -N IP-MASQ
iptables -w -t nat -A POSTROUTING -m comment --comment "ip-masq: ensure nat POSTROUTING directs all non-LOCAL destination traffic to our custom IP-MASQ chain" -m addrtype ! --dst-type LOCAL -j IP-MASQ
iptables -w -t nat -A IP-MASQ -d {{.NonMasqueradeCIDR}} -m comment --comment "ip-masq: pod cidr is not subject to MASQUERADE" -j RETURN
iptables -w -t nat -A IP-MASQ -m comment --comment "ip-masq: outbound traffic is subject to MASQUERADE (must be last in chain)" -j MASQUERADE
`

	script = strings.ReplaceAll(script, "{{.NonMasqueradeCIDR}}", b.NodeupConfig.Networking.NonMasqueradeCIDR)

	c.AddTask(&nodetasks.File{
		Path:     "/opt/kops/bin/cni-iptables-setup",
		Contents: fi.NewStringResource(script),
		Type:     nodetasks.FileType_File,
		Mode:     s("0755"),
	})

	manifest := &systemd.Manifest{}
	manifest.Set("Unit", "Description", "Configure iptables for kubernetes CNI")
	manifest.Set("Unit", "Documentation", "https://github.com/kubernetes/kops")
	manifest.Set("Unit", "Before", "network.target")
	manifest.Set("Service", "Type", "oneshot")
	manifest.Set("Service", "RemainAfterExit", "yes")
	manifest.Set("Service", "ExecStart", "/opt/kops/bin/cni-iptables-setup")
	manifest.Set("Install", "WantedBy", "basic.target")

	manifestString := manifest.Render()
	klog.V(8).Infof("Built service manifest %q\n%s", "cni-iptables-setup", manifestString)

	service := &nodetasks.Service{
		Name:       "cni-iptables-setup.service",
		Definition: s(manifestString),
	}
	service.InitDefaults()
	c.AddTask(service)

	return nil
}

// buildCNIConfigTemplateFile is responsible for creating a special template for setups using Kubenet
func (b *ContainerdBuilder) buildCNIConfigTemplateFile(c *fi.NodeupModelBuilderContext) error {
	// Based on https://github.com/kubernetes/kubernetes/blob/15a8a8ec4a3275a33b7f8eb3d4d98db2abad55b7/cluster/gce/gci/configure-helper.sh#L2911-L2937

	contents := `{
    "cniVersion": "0.4.0",
    "name": "k8s-pod-network",
    "plugins": [
        {
            "type": "ptp",
            "ipam": {
                "type": "host-local",
                "ranges": [[{"subnet": "{{.PodCIDR}}"}]],
                "routes": {{Routes}}
            }
        },
        {
            "type": "portmap",
            "capabilities": {"portMappings": true}
        }
    ]
}
`

	// We will gradually build up the schema here, as needed
	type Route struct {
		Dest string `json:"dst"`
	}

	routes := []Route{
		{Dest: "0.0.0.0/0"},
	}
	if b.IsIPv6Only() {
		routes = append(routes, Route{Dest: "::/0"})
	}
	routesJSON, err := json.Marshal(routes)
	if err != nil {
		return fmt.Errorf("building json: %w", err)
	}
	contents = strings.ReplaceAll(contents, "{{Routes}}", string(routesJSON))

	klog.V(8).Infof("Built containerd CNI config template\n%s", contents)

	c.AddTask(&nodetasks.File{
		Path:     "/etc/containerd/config-cni.template",
		Contents: fi.NewStringResource(contents),
		Type:     nodetasks.FileType_File,
	})
	return nil
}

func (b *ContainerdBuilder) buildContainerdConfig() (string, error) {
	containerd := b.NodeupConfig.ContainerdConfig
	if fi.ValueOf(containerd.ConfigOverride) != "" {
		return *containerd.ConfigOverride, nil
	}

	// Build config file for containerd running in CRI mode

	config, _ := toml.Load("")
	config.SetPath([]string{"version"}, int64(2))

	if containerd.NRI != nil && (containerd.NRI.Enabled == nil || fi.ValueOf(containerd.NRI.Enabled)) {
		config.SetPath([]string{"plugins", "io.containerd.nri.v1.nri", "disable"}, false)
		if containerd.NRI.PluginRequestTimeout != nil {
			config.SetPath([]string{"plugins", "io.containerd.nri.v1.nri", "plugin_request_timeout"}, containerd.NRI.PluginRequestTimeout)
		}
		if containerd.NRI.PluginRegistrationTimeout != nil {
			config.SetPath([]string{"plugins", "io.containerd.nri.v1.nri", "plugin_registration_timeout"}, containerd.NRI.PluginRegistrationTimeout)
		}
	}
	if containerd.SeLinuxEnabled {
		config.SetPath([]string{"plugins", "io.containerd.grpc.v1.cri", "enable_selinux"}, true)
	}
	if b.NodeupConfig.KubeletConfig.PodInfraContainerImage != "" {
		config.SetPath([]string{"plugins", "io.containerd.grpc.v1.cri", "sandbox_image"}, b.NodeupConfig.KubeletConfig.PodInfraContainerImage)
	}
	for name, endpoints := range containerd.RegistryMirrors {
		config.SetPath([]string{"plugins", "io.containerd.grpc.v1.cri", "registry", "mirrors", name, "endpoint"}, endpoints)
	}
	config.SetPath([]string{"plugins", "io.containerd.grpc.v1.cri", "containerd", "default_runtime_name"}, "runc")
	config.SetPath([]string{"plugins", "io.containerd.grpc.v1.cri", "containerd", "runtimes", "runc", "runtime_type"}, "io.containerd.runc.v2")
	config.SetPath([]string{"plugins", "io.containerd.grpc.v1.cri", "containerd", "runtimes", "runc", "options", "SystemdCgroup"}, true)
	if b.NodeupConfig.UsesKubenet {
		// Using containerd with Kubenet requires special configuration.
		// This is a temporary backwards-compatible solution for kubenet users and will be deprecated when Kubenet is deprecated:
		// https://github.com/containerd/containerd/blob/master/docs/cri/config.md#cni-config-template
		config.SetPath([]string{"plugins", "io.containerd.grpc.v1.cri", "cni", "conf_template"}, "/etc/containerd/config-cni.template")
	}

	if b.InstallNvidiaRuntime() {
		if err := appendNvidiaGPURuntimeConfig(config); err != nil {
			return "", err
		}
	}

	for k, v := range containerd.ConfigAdditions {
		r := csv.NewReader(strings.NewReader(k))
		r.Comma = '.'
		path, err := r.Read()
		if err != nil {
			return "", fmt.Errorf("parsing additional containerd config entry: %w", err)
		}

		if v.Type == intstr.Int {
			config.SetPath(path, int64(v.IntValue()))
		} else {
			if v.String() == "true" {
				config.SetPath(path, true)
			} else if v.String() == "false" {
				config.SetPath(path, false)
			} else {
				config.SetPath(path, v.String())
			}
		}
	}

	return config.String(), nil
}

func appendNvidiaGPURuntimeConfig(config *toml.Tree) error {
	gpuConfig, err := toml.TreeFromMap(
		map[string]interface{}{
			"privileged_without_host_devices": false,
			"runtime_engine":                  "",
			"runtime_root":                    "",
			"runtime_type":                    "io.containerd.runc.v2",
			"options": map[string]interface{}{
				"SystemdCgroup": true,
				"BinaryName":    "/usr/bin/nvidia-container-runtime",
			},
		},
	)
	if err != nil {
		return err
	}

	config.SetPath([]string{"plugins", "io.containerd.grpc.v1.cri", "containerd", "runtimes", "nvidia"}, gpuConfig)

	return nil
}
