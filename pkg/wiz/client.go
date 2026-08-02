package wiz

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"
)

type payload struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

type discoveryPayload struct {
	Method string            `json:"method"`
	Params map[string]string `json:"params"`
}

// PilotState describes current runtime light state from getPilot.
type PilotState struct {
	Power      bool
	Brightness int
	ColorHex   string
	Temp       int
}

// Device describes a discovered WiZ device.
type Device struct {
	IP       string
	Mac      string
	Name     string
	Model    string
	Firmware string
}

// DiscoverDevices scans local network broadcast targets and returns detected bulbs.
func DiscoverDevices() ([]Device, error) {
	listenAddr := &net.UDPAddr{IP: net.IPv4zero, Port: 0}
	conn, err := net.ListenUDP("udp4", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery socket: %w", err)
	}
	defer conn.Close()

	_ = conn.SetWriteBuffer(8 * 1024)
	_ = conn.SetReadBuffer(16 * 1024)

	discovery := discoveryPayload{Method: "getSystemConfig", Params: map[string]string{}}
	jsonData, err := json.Marshal(discovery)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal discovery payload: %w", err)
	}

	targets := discoveryTargets(38899)
	for i := 0; i < 3; i++ {
		for _, target := range targets {
			_, _ = conn.WriteToUDP(jsonData, target)
		}
		time.Sleep(150 * time.Millisecond)
	}

	_ = conn.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))

	devicesByKey := make(map[string]Device)
	buffer := make([]byte, 2048)
	for {
		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			return nil, fmt.Errorf("error reading discovery response: %w", err)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(buffer[:n], &response); err != nil {
			continue
		}

		if result, ok := response["result"].(map[string]interface{}); ok {
			mac := asString(result["mac"])
			name := asString(result["moduleName"])
			model := asString(result["moduleName"])
			firmware := asString(result["fwVersion"])

			if name == "" {
				name = asString(result["deviceName"])
			}
			if name == "" {
				name = makeFallbackName(mac, addr.IP.String())
			}

			device := Device{IP: addr.IP.String(), Mac: mac, Name: name, Model: model, Firmware: firmware}
			key := strings.ToLower(strings.TrimSpace(mac))
			if key == "" {
				key = "ip:" + device.IP
			}
			devicesByKey[key] = device
		}
	}

	devices := make([]Device, 0, len(devicesByKey))
	for _, device := range devicesByKey {
		devices = append(devices, device)
	}

	sort.Slice(devices, func(i, j int) bool {
		if devices[i].Name == devices[j].Name {
			return devices[i].IP < devices[j].IP
		}
		return devices[i].Name < devices[j].Name
	})

	return devices, nil
}

// DiscoverDeviceByMAC broadcasts a discovery packet and returns the first device
// whose MAC matches targetMac. It returns as soon as a match is found, making
// it far faster than a full scan (~15-50ms vs 1500ms).
func DiscoverDeviceByMAC(targetMac, port string, timeout time.Duration) (Device, error) {
	normalizedTarget := strings.ToLower(strings.TrimSpace(targetMac))
	if normalizedTarget == "" {
		return Device{}, fmt.Errorf("targetMac cannot be empty")
	}

	portNum := 38899
	if p, err := strconv.Atoi(port); err == nil && p > 0 {
		portNum = p
	}

	listenAddr := &net.UDPAddr{IP: net.IPv4zero, Port: 0}
	conn, err := net.ListenUDP("udp4", listenAddr)
	if err != nil {
		return Device{}, fmt.Errorf("failed to create discovery socket: %w", err)
	}
	defer conn.Close()

	_ = conn.SetWriteBuffer(8 * 1024)
	_ = conn.SetReadBuffer(16 * 1024)

	discovery := discoveryPayload{Method: "getSystemConfig", Params: map[string]string{}}
	jsonData, err := json.Marshal(discovery)
	if err != nil {
		return Device{}, fmt.Errorf("failed to marshal discovery payload: %w", err)
	}

	targets := discoveryTargets(portNum)
	for _, target := range targets {
		_, _ = conn.WriteToUDP(jsonData, target)
	}

	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}
	_ = conn.SetReadDeadline(time.Now().Add(timeout))

	buffer := make([]byte, 2048)
	for {
		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			return Device{}, fmt.Errorf("error reading discovery response: %w", err)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(buffer[:n], &response); err != nil {
			continue
		}

		if result, ok := response["result"].(map[string]interface{}); ok {
			mac := strings.ToLower(strings.TrimSpace(asString(result["mac"])))
			if mac != normalizedTarget {
				continue
			}
			name := asString(result["moduleName"])
			model := asString(result["moduleName"])
			firmware := asString(result["fwVersion"])
			if name == "" {
				name = asString(result["deviceName"])
			}
			if name == "" {
				name = makeFallbackName(mac, addr.IP.String())
			}
			return Device{IP: addr.IP.String(), Mac: mac, Name: name, Model: model, Firmware: firmware}, nil
		}
	}

	return Device{}, fmt.Errorf("device with MAC %s not found on local network", targetMac)
}

// SendCommand sends a UDP command payload with retries and timeout handling.
func SendCommand(ip, port, method string, params map[string]interface{}) error {
	payload := payload{Method: method, Params: params}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON payload: %w", err)
	}

	address := net.JoinHostPort(ip, port)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		conn, err := net.Dial("udp", address)
		if err != nil {
			lastErr = fmt.Errorf("failed to connect to %s (attempt %d): %w", address, attempt+1, err)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
			}
			continue
		}

		_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		_, err = conn.Write(jsonData)
		_ = conn.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to send data to %s (attempt %d): %w", address, attempt+1, err)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
			}
			continue
		}
		return nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("unknown UDP send failure")
	}
	return lastErr
}

// GetPilotState fetches current power, brightness, and RGB color from a device.
func GetPilotState(ip, port string) (PilotState, error) {
	request := payload{Method: "getPilot", Params: map[string]interface{}{}}
	jsonData, err := json.Marshal(request)
	if err != nil {
		return PilotState{}, fmt.Errorf("failed to marshal getPilot payload: %w", err)
	}

	address := net.JoinHostPort(ip, port)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		conn, err := net.Dial("udp", address)
		if err != nil {
			lastErr = fmt.Errorf("failed to connect to %s (attempt %d): %w", address, attempt+1, err)
			time.Sleep(time.Duration(attempt+1) * 120 * time.Millisecond)
			continue
		}

		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
		if _, err := conn.Write(jsonData); err != nil {
			_ = conn.Close()
			lastErr = fmt.Errorf("failed to send getPilot to %s (attempt %d): %w", address, attempt+1, err)
			time.Sleep(time.Duration(attempt+1) * 120 * time.Millisecond)
			continue
		}

		buffer := make([]byte, 4096)
		n, err := conn.Read(buffer)
		_ = conn.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed reading getPilot response from %s (attempt %d): %w", address, attempt+1, err)
			time.Sleep(time.Duration(attempt+1) * 120 * time.Millisecond)
			continue
		}

		var response map[string]interface{}
		if err := json.Unmarshal(buffer[:n], &response); err != nil {
			lastErr = fmt.Errorf("failed to decode getPilot response: %w", err)
			continue
		}

		result, ok := response["result"].(map[string]interface{})
		if !ok {
			lastErr = fmt.Errorf("getPilot response missing result")
			continue
		}

		power := asBool(result["state"])
		brightness := asInt(result["dimming"])
		if brightness < 0 {
			brightness = 0
		}
		if brightness > 100 {
			brightness = 100
		}

		_, hasR := result["r"]
		_, hasG := result["g"]
		_, hasB := result["b"]
		colorHex := ""
		if hasR || hasG || hasB {
			r := asInt(result["r"])
			g := asInt(result["g"])
			b := asInt(result["b"])
			colorHex = fmt.Sprintf("#%02X%02X%02X", clampColor(r), clampColor(g), clampColor(b))
		}
		temp := asInt(result["temp"])

		return PilotState{Power: power, Brightness: brightness, ColorHex: colorHex, Temp: temp}, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("unknown getPilot failure")
	}
	return PilotState{}, lastErr
}

// HexToRGB converts a six-digit hex color string to RGB values.
func HexToRGB(h string) (uint8, uint8, uint8, error) {
	h = strings.TrimPrefix(h, "#")
	if len(h) != 6 {
		return 0, 0, 0, fmt.Errorf("invalid hex")
	}
	b, err := hex.DecodeString(h)
	if err != nil {
		return 0, 0, 0, err
	}
	return b[0], b[1], b[2], nil
}

func discoveryTargets(port int) []*net.UDPAddr {
	targets := map[string]*net.UDPAddr{
		net.JoinHostPort("255.255.255.255", fmt.Sprintf("%d", port)): &net.UDPAddr{IP: net.IPv4bcast.To4(), Port: port},
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return []*net.UDPAddr{{IP: net.IPv4bcast, Port: port}}
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagBroadcast == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP == nil || ipNet.Mask == nil {
				continue
			}

			ipv4 := ipNet.IP.To4()
			if ipv4 == nil || len(ipNet.Mask) < 4 {
				continue
			}

			broadcast := make(net.IP, len(ipv4))
			for idx := 0; idx < 4; idx++ {
				broadcast[idx] = ipv4[idx] | ^ipNet.Mask[idx]
			}

			target := &net.UDPAddr{IP: broadcast, Port: port}
			targets[target.String()] = target
		}
	}

	result := make([]*net.UDPAddr, 0, len(targets))
	for _, target := range targets {
		result = append(result, target)
	}
	return result
}

func asString(value interface{}) string {
	if value == nil {
		return ""
	}
	if str, ok := value.(string); ok {
		return strings.TrimSpace(str)
	}
	return strings.TrimSpace(fmt.Sprintf("%v", value))
}

func asInt(value interface{}) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	case int32:
		return int(typed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return parsed
		}
	}
	return 0
}

func asBool(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case float64:
		return typed != 0
	case int:
		return typed != 0
	case string:
		norm := strings.ToLower(strings.TrimSpace(typed))
		return norm == "1" || norm == "true" || norm == "on"
	}
	return false
}

func clampColor(value int) int {
	if value < 0 {
		return 0
	}
	if value > 255 {
		return 255
	}
	return value
}

func makeFallbackName(mac, ip string) string {
	cleanMac := strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(mac)), ":", "")
	if len(cleanMac) >= 4 {
		return "WiZ-" + cleanMac[len(cleanMac)-4:]
	}
	if ip != "" {
		return "WiZ-" + ip
	}
	return "WiZ Device"
}
