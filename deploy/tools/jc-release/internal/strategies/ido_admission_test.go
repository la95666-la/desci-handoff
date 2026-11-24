package strategies

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCheckIDOJobLayoutRejectsAbsentAndUnsafeConfigs(t *testing.T) {
	root := t.TempDir()
	if err := CheckIDOJobLayout(root); err == nil {
		t.Fatal("missing Job config admitted")
	}
	for _, component := range []string{"job", "common", "secret"} {
		if err := os.MkdirAll(filepath.Join(root, "job-runtime", "config", component), 0700); err != nil {
			t.Fatal(err)
		}
	}
	config := "application:\n  environment: prod\n  jobMode: ido\nchain:\n  project:\n    chainID: 56\n    url: https://rpc.example.invalid\n    v1:\n      enabled: 'false'\n    v2:\n      contractAddress: 0x0000000000000000000000000000000000000002\n      coreOperatorAddress: 0x0000000000000000000000000000000000000001\nfairLaunchMainnet:\n  scanner:\n    enabled: false\n    exclusiveJobMode: false\n"
	compose := "services:\n  job:\n    container_name: jc-job\n    image: joincare/job:reviewed\n    entrypoint: ['/app/joincare-job', '--path', '/app/config/']\n    volumes: ['./job-runtime/config:/app/config:ro']\n    healthcheck:\n      test: ['CMD', 'curl', '--fail', 'http://127.0.0.1:13007/health/job-ready']\n"
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, path), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("job-runtime/config/job/config.yaml", config)
	write("job-runtime/config/common/config.yaml", "application: {}\n")
	secret := "chain:\n  project:\n    jobPrivate: isolated-test-placeholder\n"
	write("job-runtime/config/secret/config.yaml", secret)
	write("docker-compose.yml", compose)
	if err := CheckIDOJobLayout(root); err != nil {
		t.Fatalf("positive static layout rejected: %v", err)
	}
	write("job-runtime/config/job/config.yaml", strings.Replace(config, "jobMode: ido", "jobMode: ido-v3", 1))
	if err := CheckIDOJobLayout(root); err != nil {
		t.Fatalf("explicit V3-only role rejected: %v", err)
	}
	write("job-runtime/config/job/config.yaml", config)
	t.Run("unrelated service long mounts and scalar entrypoint", func(t *testing.T) {
		write("docker-compose.yml", compose+"  admin:\n    image: joincare/admin:reviewed\n    entrypoint: nginx\n    volumes:\n      - type: bind\n        source: /opt/joincare-prod/admin-nginx\n        target: /etc/nginx/conf.d\n        read_only: true\n")
		if err := CheckIDOJobLayout(root); err != nil {
			t.Fatalf("valid sibling service broke Job-only admission: %v", err)
		}
		write("docker-compose.yml", compose)
	})
	if runtime.GOOS == "linux" {
		if err := os.Chmod(root, 0777); err != nil {
			t.Fatal(err)
		}
		if err := CheckIDOJobLayout(root); err == nil {
			t.Fatal("writable deployment root admitted")
		}
		if err := os.Chmod(root, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(filepath.Join(root, "docker-compose.yml"), 0666); err != nil {
			t.Fatal(err)
		}
		if err := CheckIDOJobLayout(root); err == nil {
			t.Fatal("writable compose admitted")
		}
		if err := os.Chmod(filepath.Join(root, "docker-compose.yml"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range map[string]string{
		"fair launch role":             strings.Replace(config, "jobMode: ido", "jobMode: fair-launch", 1),
		"duplicate case job role":      config + "APPLICATION:\n  JOBMODE: fair-launch\n",
		"duplicate case core operator": config + "CHAIN:\n  PROJECT:\n    V2:\n      COREOPERATORADDRESS: 0x0000000000000000000000000000000000000001\n",
		"missing operator":             strings.Replace(config, "      coreOperatorAddress: 0x0000000000000000000000000000000000000001\n", "", 1),
		"wrong chain ID":               strings.Replace(config, "chainID: 56", "chainID: 97", 1),
		"missing RPC URL":              strings.Replace(config, "    url: https://rpc.example.invalid\n", "", 1),
		"missing contract":             strings.Replace(config, "      contractAddress: 0x0000000000000000000000000000000000000002\n", "", 1),
		"V1 enabled":                   strings.Replace(config, "enabled: 'false'", "enabled: 'true'", 1),
		"scanner enabled":              strings.Replace(config, "enabled: false\n    exclusive", "enabled: true\n    exclusive", 1),
	} {
		t.Run(name, func(t *testing.T) {
			write("job-runtime/config/job/config.yaml", body)
			if err := CheckIDOJobLayout(root); err == nil {
				t.Fatal("unsafe Job role admitted")
			}
			write("job-runtime/config/job/config.yaml", config)
		})
	}
	write("job-runtime/config/secret/config.yaml", secret+"    private: core-secret-must-not-be-present\n")
	if err := CheckIDOJobLayout(root); err == nil {
		t.Fatal("Core relayer key exposed to Job")
	}
	write("job-runtime/config/secret/config.yaml", secret)
	for _, tc := range []struct{ name, path, body string }{
		{"common overrides role", "job-runtime/config/common/config.yaml", "application:\n  jobMode: fair-launch\n"},
		{"common overrides operator", "job-runtime/config/common/config.yaml", "chain:\n  project:\n    v2:\n      coreOperatorAddress: 0x0000000000000000000000000000000000000001\n"},
		{"common overrides chain ID", "job-runtime/config/common/config.yaml", "chain:\n  project:\n    chainID: 97\n"},
		{"secret overrides contract", "job-runtime/config/secret/config.yaml", secret + "    v2:\n      contractAddress: 0x0000000000000000000000000000000000000003\n"},
		{"secret overrides role", "job-runtime/config/secret/config.yaml", secret + "application:\n  jobMode: fair-launch\n"},
		{"case folded role", "job-runtime/config/secret/config.yaml", secret + "APPLICATION:\n  jobmode: fair-launch\n"},
		{"duplicate case role", "job-runtime/config/secret/config.yaml", secret + "application: {}\nAPPLICATION:\n  jobmode: fair-launch\n"},
		{"common exposes core key", "job-runtime/config/common/config.yaml", "chain:\n  project:\n    private: forbidden\n"},
		{"case folded core key", "job-runtime/config/common/config.yaml", "CHAIN:\n  PROJECT:\n    PRIVATE: forbidden\n"},
		{"job config exposes core key", "job-runtime/config/job/config.yaml", strings.Replace(config, "    v1:\n", "    private: forbidden\n    v1:\n", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			write(tc.path, tc.body)
			if err := CheckIDOJobLayout(root); err == nil {
				t.Fatal("later Viper override or Core relayer admitted")
			}
			switch tc.path {
			case "job-runtime/config/common/config.yaml":
				write(tc.path, "application: {}\n")
			case "job-runtime/config/secret/config.yaml":
				write(tc.path, secret)
			default:
				write(tc.path, config)
			}
		})
	}
	for name, body := range map[string]string{
		"missing fixed container name": strings.Replace(compose, "    container_name: jc-job\n", "", 1),
		"long Job mount still refused": strings.Replace(compose, "volumes: ['./job-runtime/config:/app/config:ro']", "volumes: [{type: bind, source: ./job-runtime/config, target: /app/config, read_only: true}]", 1),
		"backend config mount":         strings.Replace(compose, "./job-runtime/config:/app/config:ro", "./config:/app/config:ro", 1),
		"backend elsewhere":            strings.Replace(compose, "    healthcheck:", "      - './config:/tmp/backend:ro'\n    healthcheck:", 1),
		"second config mount":          strings.Replace(compose, "    healthcheck:", "      - './config:/app/config/secret:ro'\n    healthcheck:", 1),
		"inherited env file":           strings.Replace(compose, "    healthcheck:", "    env_file: ['.deploy.env']\n    healthcheck:", 1),
		"inherited env":                strings.Replace(compose, "    healthcheck:", "    environment: {CORE_PRIVATE: example}\n    healthcheck:", 1),
		"nonexact entrypoint":          strings.Replace(compose, "'--path', '/app/config/'", "'--path', '/app/config/', '--unsafe'", 1),
		"missing readiness":            strings.Replace(compose, "/health/job-ready", "/health", 1),
		"health with extra command":    strings.Replace(compose, "'http://127.0.0.1:13007/health/job-ready']", "'http://127.0.0.1:13007/health/job-ready', 'sh', '-c', 'unsafe']", 1),
		"health disabled":              strings.Replace(compose, "      test:", "      disable: true\n      test:", 1),
		"health shell":                 strings.Replace(compose, "['CMD', 'curl'", "['CMD-SHELL', 'curl'", 1),
		"core entrypoint":              strings.Replace(compose, "/app/joincare-job", "/app/joincare-core", 1),
		"privileged job":               strings.Replace(compose, "    healthcheck:", "    privileged: true\n    healthcheck:", 1),
		"host network":                 strings.Replace(compose, "    healthcheck:", "    network_mode: host\n    healthcheck:", 1),
		"extra capabilities":           strings.Replace(compose, "    healthcheck:", "    cap_add: [SYS_ADMIN]\n    healthcheck:", 1),
		"mutable build":                strings.Replace(compose, "    healthcheck:", "    build: .\n    healthcheck:", 1),
		"attacker label":               strings.Replace(compose, "    healthcheck:", "    labels: {joincare.release.attempt: forged}\n    healthcheck:", 1),
		"merge inherited env":          "x-core: &core\n  environment: {CORE_PRIVATE: leaked}\n" + strings.Replace(compose, "    healthcheck:", "    <<: *core\n    healthcheck:", 1),
	} {
		t.Run(name, func(t *testing.T) {
			write("docker-compose.yml", body)
			if err := CheckIDOJobLayout(root); err == nil {
				t.Fatal("unsafe Job service admitted")
			}
			write("docker-compose.yml", compose)
		})
	}
}
