package main

import (
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/shivarchit/Lumina-TUI/pkg/config"
)

func loopbackUDP(t *testing.T) (port string, stop func()) {
	t.Helper()
	srv, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		buf := make([]byte, 2048)
		for {
			if _, _, e := srv.ReadFromUDP(buf); e != nil {
				return
			}
		}
	}()
	return strconv.Itoa(srv.LocalAddr().(*net.UDPAddr).Port), func() { _ = srv.Close() }
}

func TestFanoutAggregates(t *testing.T) {
	port, stop := loopbackUDP(t)
	defer stop()

	targets := []Target{
		{IP: "127.0.0.1", Port: port, Name: "A"},
		{IP: "127.0.0.1", Port: port, Name: "B"},
	}
	res := (&App{}).fanout(targets, "setState", map[string]interface{}{"state": true})
	if res.OK != 2 || len(res.Failed) != 0 {
		t.Fatalf("expected 2 ok / 0 failed, got %d/%v", res.OK, res.Failed)
	}
}

func TestUpdateSavedIPsHealsByMAC(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // persist() writes $HOME/.lumina-config.json
	a := &App{cfg: config.Config{SavedDevices: []config.SavedDevice{
		{Name: "Desk", IP: "10.0.0.2", Mac: "aa:aa"},
		{Name: "Shelf", IP: "10.0.0.3", Mac: "bb:bb"},
	}}}
	if !a.updateSavedIPs(map[string]string{"aa:aa": "10.0.0.9"}) {
		t.Fatal("changed IP must report true")
	}
	if a.cfg.SavedDevices[0].IP != "10.0.0.9" || a.cfg.SavedDevices[1].IP != "10.0.0.3" {
		t.Fatalf("bad heal: %+v", a.cfg.SavedDevices)
	}
	if a.updateSavedIPs(map[string]string{"aa:aa": "10.0.0.9", "cc:cc": "10.0.0.1"}) {
		t.Fatal("same IP / unknown MAC must report false")
	}
}

func TestGroupTargetsResolvesAndFallsBack(t *testing.T) {
	a := &App{cfg: config.Config{
		Port: "38899",
		SavedDevices: []config.SavedDevice{
			{Name: "Desk", IP: "10.0.0.2", Port: "38899", Mac: "aa:aa"},
			{Name: "Shelf", IP: "10.0.0.3", Mac: "bb:bb"},
		},
		Groups: []config.Group{{Name: "Room", Macs: []string{"AA:AA", "bb:bb", "cc:cc"}}},
	}}

	targets := a.GroupTargets("Room")
	if len(targets) != 2 {
		t.Fatalf("expected 2 resolvable members, got %d", len(targets))
	}
	if targets[0].IP != "10.0.0.2" || targets[1].Port != "38899" {
		t.Fatalf("bad resolution: %+v", targets)
	}
	if got := a.GroupTargets("nope"); len(got) != 0 {
		t.Fatalf("unknown group must resolve empty, got %+v", got)
	}
}

func TestSaveGroupRejectsEmptyName(t *testing.T) {
	a := &App{}
	if err := a.SaveGroup(config.Group{Name: "  "}); err == nil {
		t.Fatal("expected error for empty group name")
	}
}

func TestHintForLocalNetworkDenial(t *testing.T) {
	err := fmt.Errorf("write udp4 0.0.0.0:52341->10.0.0.2:38899: sendto: no route to host")
	if hintFor(err) == "" {
		t.Fatal("expected a Local Network hint for EHOSTUNREACH")
	}
	if hintFor(fmt.Errorf("read: i/o timeout")) != "" {
		t.Fatal("plain timeout must not claim a permission problem")
	}
	if hintFor(nil) != "" {
		t.Fatal("nil error must yield no hint")
	}
}

func TestLnPromptDebounce(t *testing.T) {
	var p lnPrompt
	t0 := time.Now()
	if prime, pane := p.due(t0); !prime || !pane {
		t.Fatal("first failure must fire both prime and pane")
	}
	if prime, pane := p.due(t0.Add(5 * time.Second)); prime || pane {
		t.Fatal("5s later: both debounced")
	}
	if prime, pane := p.due(t0.Add(16 * time.Second)); !prime || pane {
		t.Fatal("16s later: prime refires, pane still debounced")
	}
	if prime, pane := p.due(t0.Add(80 * time.Second)); !prime || !pane {
		t.Fatal("80s later: both refire")
	}
}

func TestSaveDeviceRequiresMAC(t *testing.T) {
	a := &App{}
	if err := a.SaveDevice(config.SavedDevice{Name: "x", IP: "1.2.3.4"}); err == nil {
		t.Fatal("expected error for missing MAC")
	}
}
