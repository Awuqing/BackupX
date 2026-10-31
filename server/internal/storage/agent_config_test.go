package storage

import (
	"encoding/json"
	"testing"
)

func TestConfigForAgentPreservesTransferFieldsAndRemovesMonitoring(t *testing.T) {
	raw := []byte(`{"root":"/backups","masterRelay":true,"customNumber":9007199254740993,"secretAccessKey":"transfer-secret","minioCapacity":true,"minioMetricsToken":"monitoring-token"}`)
	result, err := ConfigForAgent(raw)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]json.RawMessage
	if err := json.Unmarshal(result, &config); err != nil {
		t.Fatal(err)
	}
	if config["minioCapacity"] != nil || config["minioMetricsToken"] != nil {
		t.Fatal("monitoring configuration was forwarded")
	}
	if string(config["root"]) != `"/backups"` || string(config["masterRelay"]) != "true" || string(config["customNumber"]) != "9007199254740993" || string(config["secretAccessKey"]) != `"transfer-secret"` {
		t.Fatal("transfer configuration changed")
	}
	if _, err := ConfigForAgent([]byte("invalid")); err == nil {
		t.Fatal("invalid storage configuration was accepted")
	}
}
