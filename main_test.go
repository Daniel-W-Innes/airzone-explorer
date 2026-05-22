package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestReadSecretFileTrimsWhitespace(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "password.txt")
	if err := os.WriteFile(path, []byte(" secretpassword1234 \n"), 0o600); err != nil {
		t.Fatalf("write password file: %v", err)
	}

	got, err := readSecretFile(path)
	if err != nil {
		t.Fatalf("readSecretFile returned error: %v", err)
	}
	if got != "secretpassword1234" {
		t.Fatalf("readSecretFile() = %q, want %q", got, "secretpassword1234")
	}
}

func TestValidateConfigRequiresInputs(t *testing.T) {
	t.Parallel()

	err := validateConfig(exporterConfig{})
	if err == nil {
		t.Fatal("validateConfig() error = nil, want non-nil")
	}
}

func TestGetDeviceConfigUsesInstallationQuery(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/devices/device-1/config" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/devices/device-1/config")
		}
		if got := r.URL.Query().Get("installation_id"); got != "installation-1" {
			t.Fatalf("installation_id = %q, want %q", got, "installation-1")
		}
		if got := r.URL.Query().Get("type"); got != "user" {
			t.Fatalf("type = %q, want %q", got, "user")
		}
		if got := r.Header.Get("Authorization"); got != "Bearer jwt-token" {
			t.Fatalf("authorization = %q, want %q", got, "Bearer jwt-token")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"leds_active":true}`))
	}))
	defer server.Close()

	got, err := getDeviceConfig(context.Background(), server.Client(), server.URL, "jwt-token", "installation-1", "device-1")
	if err != nil {
		t.Fatalf("getDeviceConfig returned error: %v", err)
	}
	if !strings.Contains(string(got), `"leds_active":true`) {
		t.Fatalf("config = %s, expected leds_active", string(got))
	}
}

func TestCollectorExportsDeviceMetrics(t *testing.T) {
	t.Parallel()

	passwordPath := filepath.Join(t.TempDir(), "password.txt")
	if err := os.WriteFile(passwordPath, []byte("secretpassword1234\n"), 0o600); err != nil {
		t.Fatalf("write password file: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/auth/login":
			_, _ = w.Write([]byte(`{"token":"jwt-token","refreshToken":"refresh-token"}`))
		case "/installations":
			if got := r.Header.Get("Authorization"); got != "Bearer jwt-token" {
				t.Fatalf("authorization = %q, want %q", got, "Bearer jwt-token")
			}
			_, _ = w.Write([]byte(`{"total":1,"pendingInstallations":null,"installations":[{"installation_id":"installation-1","location_id":"location-1","name":"Home","access_type":"admin","ws_ids":["ws-1"]}]}`))
		case "/installations/installation-1":
			_, _ = w.Write([]byte(`{"_id":"relation-1","user_id":"user-1","installation_id":"installation-1","location_id":"location-1","confirmation_date":null,"groups":[{"group_id":"group-1","name":"Living Area","icon":null,"devices":[{"device_id":"device-1","name":"Zone 1","type":"az_zone","config":null,"ws_id":"ws-1","meta":{"system_number":1,"zone_number":1}}]}],"name":"Home","access_type":"admin"}`))
		case "/devices/device-1/status":
			if got := r.URL.Query().Get("installation_id"); got != "installation-1" {
				t.Fatalf("status installation_id = %q, want %q", got, "installation-1")
			}
			_, _ = w.Write([]byte(`{"power":true,"mode":3,"humidity":45,"local_temp":{"celsius":21.5,"fah":70.7},"setpoint_air_heat":{"celsius":19,"fah":66.2}}`))
		case "/devices/device-1/config":
			if got := r.URL.Query().Get("installation_id"); got != "installation-1" {
				t.Fatalf("config installation_id = %q, want %q", got, "installation-1")
			}
			if got := r.URL.Query().Get("type"); got != "user" {
				t.Fatalf("config type = %q, want %q", got, "user")
			}
			_, _ = w.Write([]byte(`{"leds_active":true,"buttons_order":["power","mode"]}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := &airzoneClient{
		baseURL:      server.URL,
		email:        "user@example.com",
		passwordFile: passwordPath,
		httpClient:   &http.Client{Timeout: time.Second},
	}

	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(newAirzoneCollector(client))

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather returned error: %v", err)
	}

	assertGaugeValue(t, families, "airzone_up", nil, 1)
	assertGaugeValue(t, families, "airzone_devices", nil, 1)
	assertGaugeValue(t, families, "airzone_device_info", map[string]string{
		"installation_id": "installation-1",
		"device_id":       "device-1",
		"device_name":     "Zone 1",
	}, 1)
	assertGaugeValue(t, families, "airzone_device_power_ratio", map[string]string{
		"device_id": "device-1",
	}, 1)
	assertGaugeValue(t, families, "airzone_device_mode", map[string]string{
		"device_id": "device-1",
	}, 3)
	assertGaugeValue(t, families, "airzone_device_local_temperature_celsius", map[string]string{
		"device_id": "device-1",
	}, 21.5)
	assertGaugeValue(t, families, "airzone_device_humidity_ratio", map[string]string{
		"device_id": "device-1",
	}, 0.45)
	assertGaugeValue(t, families, "airzone_device_leds_active_ratio", map[string]string{
		"device_id": "device-1",
	}, 1)
	assertGaugeValue(t, families, "airzone_device_setpoint_temperature_celsius", map[string]string{
		"device_id":     "device-1",
		"setpoint_type": "heat",
	}, 19)
	assertMetricAbsent(t, families, "airzone_device_numeric_value")
	assertMetricAbsent(t, families, "airzone_device_state")
}

func assertGaugeValue(t *testing.T, families []*dto.MetricFamily, familyName string, labels map[string]string, want float64) {
	t.Helper()

	for _, family := range families {
		if family.GetName() != familyName {
			continue
		}

		for _, metric := range family.GetMetric() {
			if !metricLabelsMatch(metric, labels) {
				continue
			}

			got := metric.GetGauge().GetValue()
			if got != want {
				t.Fatalf("%s labels=%v value = %v, want %v", familyName, labels, got, want)
			}
			return
		}
	}

	t.Fatalf("metric not found: %s labels=%v", familyName, labels)
}

func assertMetricAbsent(t *testing.T, families []*dto.MetricFamily, familyName string) {
	t.Helper()

	for _, family := range families {
		if family.GetName() == familyName {
			t.Fatalf("unexpected metric family present: %s", familyName)
		}
	}
}

func metricLabelsMatch(metric *dto.Metric, labels map[string]string) bool {
	if len(labels) == 0 {
		return true
	}

	actual := make(map[string]string, len(metric.GetLabel()))
	for _, label := range metric.GetLabel() {
		actual[label.GetName()] = label.GetValue()
	}

	for key, value := range labels {
		if actual[key] != value {
			return false
		}
	}

	return true
}
