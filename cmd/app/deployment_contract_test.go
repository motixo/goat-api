package main

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
)

const (
	dockerfilePath         = "../../Dockerfile"
	kubernetesManifestPath = "../../deploy/kubernetes/goat-api.yaml"
)

type kubernetesProbe struct {
	HTTPGet struct {
		Path string `yaml:"path"`
		Port string `yaml:"port"`
	} `yaml:"httpGet"`
	PeriodSeconds    int `yaml:"periodSeconds"`
	TimeoutSeconds   int `yaml:"timeoutSeconds"`
	FailureThreshold int `yaml:"failureThreshold"`
}

type kubernetesDeployment struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Replicas int `yaml:"replicas"`
		Strategy struct {
			Type          string `yaml:"type"`
			RollingUpdate struct {
				MaxUnavailable int `yaml:"maxUnavailable"`
				MaxSurge       int `yaml:"maxSurge"`
			} `yaml:"rollingUpdate"`
		} `yaml:"strategy"`
		Template struct {
			Spec struct {
				AutomountServiceAccountToken  bool  `yaml:"automountServiceAccountToken"`
				TerminationGracePeriodSeconds int64 `yaml:"terminationGracePeriodSeconds"`
				Containers                    []struct {
					Name  string `yaml:"name"`
					Image string `yaml:"image"`
					Env   []struct {
						Name  string `yaml:"name"`
						Value string `yaml:"value"`
					} `yaml:"env"`
					Ports []struct {
						Name          string `yaml:"name"`
						ContainerPort int    `yaml:"containerPort"`
					} `yaml:"ports"`
					EnvFrom []struct {
						ConfigMapRef *struct {
							Name string `yaml:"name"`
						} `yaml:"configMapRef"`
						SecretRef *struct {
							Name string `yaml:"name"`
						} `yaml:"secretRef"`
					} `yaml:"envFrom"`
					Lifecycle struct {
						PreStop struct {
							Exec struct {
								Command []string `yaml:"command"`
							} `yaml:"exec"`
						} `yaml:"preStop"`
					} `yaml:"lifecycle"`
					StartupProbe   kubernetesProbe `yaml:"startupProbe"`
					LivenessProbe  kubernetesProbe `yaml:"livenessProbe"`
					ReadinessProbe kubernetesProbe `yaml:"readinessProbe"`
				} `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

type kubernetesService struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Ports []struct {
			Name       string `yaml:"name"`
			Port       int    `yaml:"port"`
			TargetPort string `yaml:"targetPort"`
		} `yaml:"ports"`
	} `yaml:"spec"`
}

func TestDockerRuntimeLifecycleContract(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile(dockerfilePath)
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}

	instructions := parseDockerInstructions(t, string(contents))
	assertSingleDockerInstruction(t, instructions, "STOPSIGNAL", "STOPSIGNAL SIGTERM")
	assertSingleDockerInstruction(t, instructions, "USER", "USER 10001:10001")
	assertSingleDockerInstruction(t, instructions, "ENTRYPOINT", `ENTRYPOINT ["/app/app"]`)

	healthcheck := singleDockerInstruction(t, instructions, "HEALTHCHECK")
	for _, required := range []string{
		"/api/ready",
		"${SERVER_PORT}",
		"--start-period=150s",
		"--timeout=3s",
		"--retries=3",
		"wget",
	} {
		if !strings.Contains(healthcheck, required) {
			t.Errorf("HEALTHCHECK must contain %q; got %q", required, healthcheck)
		}
	}
}

func TestKubernetesDeploymentLifecycleContract(t *testing.T) {
	t.Parallel()

	deployment, service := readKubernetesManifest(t)
	if deployment.APIVersion != "apps/v1" || deployment.Kind != "Deployment" {
		t.Fatalf("first manifest must be an apps/v1 Deployment; got %s %s", deployment.APIVersion, deployment.Kind)
	}
	if deployment.Metadata.Name != "goat-api" {
		t.Errorf("deployment name = %q, want goat-api", deployment.Metadata.Name)
	}
	if deployment.Spec.Replicas < 2 {
		t.Errorf("deployment replicas = %d, want at least 2", deployment.Spec.Replicas)
	}
	if deployment.Spec.Strategy.Type != "RollingUpdate" ||
		deployment.Spec.Strategy.RollingUpdate.MaxUnavailable != 0 ||
		deployment.Spec.Strategy.RollingUpdate.MaxSurge < 1 {
		t.Errorf("deployment must retain capacity during rolling updates: %+v", deployment.Spec.Strategy)
	}
	if deployment.Spec.Template.Spec.AutomountServiceAccountToken {
		t.Error("application pod must not mount a service-account token")
	}
	if len(deployment.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("container count = %d, want 1", len(deployment.Spec.Template.Spec.Containers))
	}

	container := deployment.Spec.Template.Spec.Containers[0]
	if container.Name != "api" {
		t.Errorf("container name = %q, want api", container.Name)
	}
	if strings.HasSuffix(container.Image, ":latest") || !strings.Contains(container.Image, ":") {
		t.Errorf("container image must use an explicit non-latest release tag; got %q", container.Image)
	}
	if len(container.Ports) != 1 || container.Ports[0].Name != "http" || container.Ports[0].ContainerPort != 8080 {
		t.Errorf("container must expose named port http:8080; got %+v", container.Ports)
	}
	assertContainerEnvironment(t, container.Env, "SERVER_PORT", "8080")
	assertExternalEnvironmentSources(t, container.EnvFrom)

	assertHTTPProbe(t, "startup", container.StartupProbe, "/api/health", "http")
	assertHTTPProbe(t, "liveness", container.LivenessProbe, "/api/health", "http")
	assertHTTPProbe(t, "readiness", container.ReadinessProbe, "/api/ready", "http")

	startupBudget := time.Duration(container.StartupProbe.PeriodSeconds*container.StartupProbe.FailureThreshold) * time.Second
	if startupBudget < 150*time.Second {
		t.Errorf("startup probe budget = %s, want at least 150s", startupBudget)
	}
	if container.ReadinessProbe.TimeoutSeconds < 3 {
		t.Errorf("readiness probe timeout = %ds, want at least 3s for the application's default 2s dependency deadline", container.ReadinessProbe.TimeoutSeconds)
	}

	preStopDelay := preStopDelaySeconds(t, container.Lifecycle.PreStop.Exec.Command)
	minimumGrace := int64(defaultShutdownTimeout/time.Second) + preStopDelay + 5
	if deployment.Spec.Template.Spec.TerminationGracePeriodSeconds < minimumGrace {
		t.Errorf(
			"termination grace = %ds, want at least %ds for preStop, bounded application shutdown, and margin",
			deployment.Spec.Template.Spec.TerminationGracePeriodSeconds,
			minimumGrace,
		)
	}

	if service.APIVersion != "v1" || service.Kind != "Service" || service.Metadata.Name != "goat-api" {
		t.Fatalf("second manifest must be the goat-api v1 Service; got %s %s %q", service.APIVersion, service.Kind, service.Metadata.Name)
	}
	if len(service.Spec.Ports) != 1 || service.Spec.Ports[0].Name != "http" ||
		service.Spec.Ports[0].Port != 80 || service.Spec.Ports[0].TargetPort != "http" {
		t.Errorf("service must route port 80 to the named HTTP container port; got %+v", service.Spec.Ports)
	}
}

func assertContainerEnvironment(t *testing.T, environment []struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}, name, wantValue string) {
	t.Helper()
	for _, variable := range environment {
		if variable.Name == name {
			if variable.Value != wantValue {
				t.Errorf("container environment %s = %q, want %q", name, variable.Value, wantValue)
			}
			return
		}
	}
	t.Errorf("container environment does not define %s", name)
}

func parseDockerInstructions(t *testing.T, contents string) []string {
	t.Helper()

	var instructions []string
	var current string
	scanner := bufio.NewScanner(strings.NewReader(contents))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		continued := strings.HasSuffix(line, "\\")
		line = strings.TrimSpace(strings.TrimSuffix(line, "\\"))
		if current == "" {
			current = line
		} else {
			current += " " + line
		}
		if !continued {
			instructions = append(instructions, current)
			current = ""
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan Dockerfile: %v", err)
	}
	if current != "" {
		t.Fatal("Dockerfile ends with an unterminated continuation")
	}
	return instructions
}

func singleDockerInstruction(t *testing.T, instructions []string, name string) string {
	t.Helper()

	prefix := name + " "
	var matches []string
	for _, instruction := range instructions {
		if strings.HasPrefix(instruction, prefix) {
			matches = append(matches, instruction)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("%s instruction count = %d, want 1", name, len(matches))
	}
	return matches[0]
}

func assertSingleDockerInstruction(t *testing.T, instructions []string, name, want string) {
	t.Helper()
	if got := singleDockerInstruction(t, instructions, name); got != want {
		t.Errorf("%s = %q, want %q", name, got, want)
	}
}

func readKubernetesManifest(t *testing.T) (kubernetesDeployment, kubernetesService) {
	t.Helper()

	file, err := os.Open(kubernetesManifestPath)
	if err != nil {
		t.Fatalf("open Kubernetes manifest: %v", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("close Kubernetes manifest: %v", closeErr)
		}
	}()

	decoder := yaml.NewDecoder(file)
	var deployment kubernetesDeployment
	if err := decoder.Decode(&deployment); err != nil {
		t.Fatalf("decode Kubernetes Deployment: %v", err)
	}
	var service kubernetesService
	if err := decoder.Decode(&service); err != nil {
		t.Fatalf("decode Kubernetes Service: %v", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("manifest must contain exactly two documents; got trailing decode error %v", err)
	}
	return deployment, service
}

func assertExternalEnvironmentSources(t *testing.T, sources []struct {
	ConfigMapRef *struct {
		Name string `yaml:"name"`
	} `yaml:"configMapRef"`
	SecretRef *struct {
		Name string `yaml:"name"`
	} `yaml:"secretRef"`
}) {
	t.Helper()

	var configMapName, secretName string
	for _, source := range sources {
		if source.ConfigMapRef != nil {
			configMapName = source.ConfigMapRef.Name
		}
		if source.SecretRef != nil {
			secretName = source.SecretRef.Name
		}
	}
	if configMapName != "goat-api-config" || secretName != "goat-api-secrets" {
		t.Errorf("environment sources = config map %q, secret %q; want goat-api-config and goat-api-secrets", configMapName, secretName)
	}
}

func assertHTTPProbe(t *testing.T, name string, probe kubernetesProbe, wantPath, wantPort string) {
	t.Helper()
	if probe.HTTPGet.Path != wantPath || probe.HTTPGet.Port != wantPort {
		t.Errorf("%s probe = %s on %s, want %s on %s", name, probe.HTTPGet.Path, probe.HTTPGet.Port, wantPath, wantPort)
	}
	if probe.PeriodSeconds <= 0 || probe.TimeoutSeconds <= 0 || probe.FailureThreshold <= 0 {
		t.Errorf("%s probe bounds must be positive: %+v", name, probe)
	}
}

func preStopDelaySeconds(t *testing.T, command []string) int64 {
	t.Helper()
	if len(command) != 3 || command[0] != "/bin/sh" || command[1] != "-c" {
		t.Fatalf("preStop command must be /bin/sh -c with a bounded sleep; got %q", command)
	}
	fields := strings.Fields(command[2])
	if len(fields) != 2 || fields[0] != "sleep" {
		t.Fatalf("preStop command must be a bounded sleep; got %q", command[2])
	}
	delay, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil || delay <= 0 {
		t.Fatalf("preStop delay must be a positive integer; got %q", fields[1])
	}
	return delay
}
