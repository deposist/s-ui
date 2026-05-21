package service

import (
	"strings"
	"testing"
)

func TestConfigCoreMethodsHandleNilCore(t *testing.T) {
	t.Cleanup(ReplaceDefaultRuntimeForTest(NewRuntimeWithCoreProvider(nil)))

	configService := &ConfigService{}
	if configService.IsCoreRunning() {
		t.Fatal("nil core should not report running")
	}

	tests := map[string]func() error{
		"StartCore":   configService.StartCore,
		"RestartCore": configService.RestartCore,
		"StopCore":    configService.StopCore,
	}
	for name, call := range tests {
		err := call()
		if err == nil || !strings.Contains(err.Error(), "core not initialized") {
			t.Fatalf("%s returned %v, want core not initialized", name, err)
		}
	}
}
