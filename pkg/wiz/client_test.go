package wiz_test

import (
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"Lumina/pkg/wiz"
)

func TestHexToRGB(t *testing.T) {
	r, g, b, err := wiz.HexToRGB("#CBA6F7")
	if err != nil {
		t.Fatalf("expected valid hex parse, got error: %v", err)
	}

	if r != 0xCB || g != 0xA6 || b != 0xF7 {
		t.Fatalf("unexpected rgb values: got (%d, %d, %d)", r, g, b)
	}
}

func TestHexToRGBInvalid(t *testing.T) {
	if _, _, _, err := wiz.HexToRGB("#FFF"); err == nil {
		t.Fatal("expected error for short hex")
	}
}

func TestGetPilotState(t *testing.T) {
	server, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("failed to start udp server: %v", err)
	}
	defer server.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		n, addr, readErr := server.ReadFromUDP(buf)
		if readErr != nil || n == 0 {
			return
		}

		response := `{"result":{"state":true,"dimming":42,"r":10,"g":20,"b":30}}`
		_, _ = server.WriteToUDP([]byte(response), addr)
	}()

	port := strconv.Itoa(server.LocalAddr().(*net.UDPAddr).Port)
	state, err := wiz.GetPilotState("127.0.0.1", port)
	if err != nil {
		t.Fatalf("GetPilotState failed: %v", err)
	}

	if !state.Power {
		t.Fatal("expected power=true")
	}
	if state.Brightness != 42 {
		t.Fatalf("expected brightness=42, got %d", state.Brightness)
	}
	if state.ColorHex != "#0A141E" {
		t.Fatalf("expected color #0A141E, got %s", state.ColorHex)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("udp responder did not complete")
	}
}

func TestGetPilotStateMissingResult(t *testing.T) {
	server, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("failed to start udp server: %v", err)
	}
	defer server.Close()

	go func() {
		buf := make([]byte, 4096)
		for i := 0; i < 3; i++ {
			_ = server.SetReadDeadline(time.Now().Add(2 * time.Second))
			_, addr, readErr := server.ReadFromUDP(buf)
			if readErr != nil {
				return
			}
			_, _ = server.WriteToUDP([]byte(`{"ok":true}`), addr)
		}
	}()

	port := strconv.Itoa(server.LocalAddr().(*net.UDPAddr).Port)
	_, err = wiz.GetPilotState("127.0.0.1", port)
	if err == nil {
		t.Fatal("expected error when getPilot response has no result")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "result") {
		t.Fatalf("expected missing result error, got: %v", err)
	}
}

func TestGetPilotStateWhiteMode(t *testing.T) {
	server, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("failed to start udp server: %v", err)
	}
	defer server.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		n, addr, readErr := server.ReadFromUDP(buf)
		if readErr != nil || n == 0 {
			return
		}
		response := `{"result":{"state":true,"dimming":50,"temp":4000}}`
		_, _ = server.WriteToUDP([]byte(response), addr)
	}()

	port := strconv.Itoa(server.LocalAddr().(*net.UDPAddr).Port)
	state, err := wiz.GetPilotState("127.0.0.1", port)
	if err != nil {
		t.Fatalf("GetPilotState failed: %v", err)
	}

	if state.ColorHex != "" {
		t.Fatalf("expected empty ColorHex in white mode, got %q", state.ColorHex)
	}
	if state.Temp != 4000 {
		t.Fatalf("expected temp 4000, got %d", state.Temp)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("udp responder did not complete")
	}
}
