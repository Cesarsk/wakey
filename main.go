package main

import (
	"bytes"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

//go:embed web/index.html
var webFS embed.FS

type config struct {
	ListenAddr     string
	TargetName     string
	TargetMAC      net.HardwareAddr
	BroadcastAddr  string
	Port           int
	AuthToken      string
	HostHelperURL   string
	HostHelperToken string
	ButtonLabel    string
	SuccessMessage string
}

type pageData struct {
	TargetName    string
	ButtonLabel   string
	APIPath       string
	TokenSet      bool
	TargetMAC     string
	BroadcastAddr string
	Port          int
}

type wakeResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type wakeRequest struct {
	TargetName    string `json:"targetName"`
	MACAddress    string `json:"macAddress"`
	BroadcastAddr string `json:"broadcastAddress"`
	Port          int    `json:"port"`
}

type wakeTarget struct {
	TargetName    string
	TargetMAC     net.HardwareAddr
	BroadcastAddr string
	Port          int
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	tpl, err := template.ParseFS(webFS, "web/index.html")
	if err != nil {
		log.Fatalf("template error: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		data := pageData{
			TargetName:    cfg.TargetName,
			ButtonLabel:   cfg.ButtonLabel,
			APIPath:       "/api/wake",
			TokenSet:      cfg.AuthToken != "",
			TargetMAC:     cfg.TargetMAC.String(),
			BroadcastAddr: cfg.BroadcastAddr,
			Port:          cfg.Port,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tpl.Execute(w, data); err != nil {
			http.Error(w, "template render failed", http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("POST /api/wake", func(w http.ResponseWriter, r *http.Request) {
		if err := authorize(r, cfg.AuthToken); err != nil {
			writeJSON(w, http.StatusUnauthorized, wakeResponse{OK: false, Message: err.Error()})
			return
		}

		target, err := resolveTarget(r, cfg)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, wakeResponse{OK: false, Message: err.Error()})
			return
		}

		log.Printf("sending magic packet to %s via %s:%d (%s)", target.TargetName, target.BroadcastAddr, target.Port, target.TargetMAC.String())

		if err := dispatchMagicPacket(target, cfg); err != nil {
			log.Printf("wake failed: %v", err)
			writeJSON(w, http.StatusInternalServerError, wakeResponse{OK: false, Message: "Failed to send magic packet."})
			return
		}

		message := cfg.SuccessMessage
		if target.TargetName != "" && target.TargetName != cfg.TargetName {
			message = fmt.Sprintf("Magic packet sent to %s.", target.TargetName)
		}

		writeJSON(w, http.StatusOK, wakeResponse{OK: true, Message: message})
	})

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           loggingMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("wakey listening on %s for %s via %s:%d", cfg.ListenAddr, cfg.TargetName, cfg.BroadcastAddr, cfg.Port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server error: %v", err)
	}
}

func loadConfig() (config, error) {
	listenAddr := getenv("LISTEN_ADDR", ":8787")
	targetName := getenv("WOL_TARGET_NAME", "My Computer")
	var mac net.HardwareAddr
	macValue := strings.TrimSpace(os.Getenv("WOL_MAC"))
	if macValue != "" {
		parsedMAC, err := net.ParseMAC(macValue)
		if err != nil {
			return config{}, fmt.Errorf("invalid WOL_MAC: %w", err)
		}
		mac = parsedMAC
	}

	portValue := getenv("WOL_PORT", "9")
	port, err := strconv.Atoi(portValue)
	if err != nil || port < 1 || port > 65535 {
		return config{}, errors.New("WOL_PORT must be a valid UDP port")
	}

	broadcastAddr := getenv("WOL_BROADCAST_ADDR", "255.255.255.255")
	if ip := net.ParseIP(broadcastAddr); ip == nil {
		return config{}, errors.New("WOL_BROADCAST_ADDR must be a valid IP address")
	}

	return config{
		ListenAddr:     listenAddr,
		TargetName:     targetName,
		TargetMAC:      mac,
		BroadcastAddr:  broadcastAddr,
		Port:           port,
		AuthToken:       os.Getenv("WAKEY_AUTH_TOKEN"),
		HostHelperURL:   strings.TrimSpace(os.Getenv("WAKEY_HOST_HELPER_URL")),
		HostHelperToken: strings.TrimSpace(os.Getenv("WAKEY_HOST_HELPER_TOKEN")),
		ButtonLabel:     getenv("WAKEY_BUTTON_LABEL", "Wake Computer"),
		SuccessMessage:  getenv("WAKEY_SUCCESS_MESSAGE", "Magic packet sent."),
	}, nil
}

func dispatchMagicPacket(target wakeTarget, cfg config) error {
	if cfg.HostHelperURL != "" {
		return sendViaHostHelper(cfg.HostHelperURL, cfg.HostHelperToken, target)
	}

	return sendMagicPacket(target.BroadcastAddr, target.Port, target.TargetMAC)
}

func sendViaHostHelper(helperURL string, helperToken string, target wakeTarget) error {
	body, err := json.Marshal(wakeRequest{
		TargetName:    target.TargetName,
		MACAddress:    target.TargetMAC.String(),
		BroadcastAddr: target.BroadcastAddr,
		Port:          target.Port,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, helperURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if helperToken != "" {
		req.Header.Set("X-Wakey-Helper-Token", helperToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("host helper returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	return nil
}

func authorize(r *http.Request, expectedToken string) error {
	if expectedToken == "" {
		return nil
	}

	token := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}

	if token == "" {
		token = strings.TrimSpace(r.URL.Query().Get("token"))
	}

	if token == "" {
		return errors.New("Missing auth token.")
	}

	if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
		return errors.New("Invalid auth token.")
	}

	return nil
}

func resolveTarget(r *http.Request, cfg config) (wakeTarget, error) {
	target := wakeTarget{
		TargetName:    cfg.TargetName,
		TargetMAC:     cfg.TargetMAC,
		BroadcastAddr: cfg.BroadcastAddr,
		Port:          cfg.Port,
	}

	if r.ContentLength == 0 {
		return target, nil
	}

	var req wakeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return wakeTarget{}, errors.New("Invalid request body.")
	}

	if name := strings.TrimSpace(req.TargetName); name != "" {
		target.TargetName = name
	}

	if macValue := strings.TrimSpace(req.MACAddress); macValue != "" {
		mac, err := net.ParseMAC(macValue)
		if err != nil {
			return wakeTarget{}, errors.New("Invalid MAC address.")
		}
		target.TargetMAC = mac
	}

	if broadcastAddr := strings.TrimSpace(req.BroadcastAddr); broadcastAddr != "" {
		if ip := net.ParseIP(broadcastAddr); ip == nil {
			return wakeTarget{}, errors.New("Invalid broadcast address.")
		}
		target.BroadcastAddr = broadcastAddr
	}

	if req.Port != 0 {
		if req.Port < 1 || req.Port > 65535 {
			return wakeTarget{}, errors.New("Invalid UDP port.")
		}
		target.Port = req.Port
	}

	if len(target.TargetMAC) == 0 {
		return wakeTarget{}, errors.New("MAC address is required.")
	}

	return target, nil
}

func sendMagicPacket(broadcastAddr string, port int, mac net.HardwareAddr) error {
	payload := make([]byte, 0, 6+16*len(mac))
	payload = append(payload, bytes.Repeat([]byte{0xFF}, 6)...)
	for range 16 {
		payload = append(payload, mac...)
	}

	addr := &net.UDPAddr{IP: net.ParseIP(broadcastAddr), Port: port}
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	rawConn, err := conn.SyscallConn()
	if err != nil {
		return err
	}

	var sockErr error
	if err := rawConn.Control(func(fd uintptr) {
		sockErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
	}); err != nil {
		return err
	}
	if sockErr != nil {
		return sockErr
	}

	if err := conn.SetWriteBuffer(len(payload)); err != nil {
		return err
	}

	for i := range 3 {
		if _, err := conn.Write(payload); err != nil {
			return err
		}
		if i < 2 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	return nil
}

func writeJSON(w http.ResponseWriter, status int, body wakeResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
