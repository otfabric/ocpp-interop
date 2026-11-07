// SPDX-License-Identifier: MIT
package main

// The control socket (docs/CONTAINER_CONTRACT.md, `call`): one JSON request
// line in, one result document line out.
//
//	{"station": "ID" | null, "action": "Reset", "payload": {...}, "timeoutMs": 10000}

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

type controlRequest struct {
	Station   *string        `json:"station"`
	Action    string         `json:"action"`
	Payload   map[string]any `json:"payload"`
	TimeoutMs int64          `json:"timeoutMs"`
}

type controlHandler func(station *string, action string, payload map[string]any, timeoutMs int64) resultDoc

func serveControl(path string, handle controlHandler) (net.Listener, error) {
	_ = os.Remove(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go serveOne(conn, handle)
		}
	}()
	return listener, nil
}

func serveOne(conn net.Conn, handle controlHandler) {
	defer conn.Close()
	reader := bufio.NewReaderSize(conn, 1<<20)
	line, err := reader.ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return
	}
	var request controlRequest
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.UseNumber()
	if err := dec.Decode(&request); err != nil {
		fmt.Fprintf(os.Stderr, "control socket: %v\n", err)
		return
	}
	if request.Payload == nil {
		request.Payload = map[string]any{}
	}
	if request.TimeoutMs <= 0 {
		request.TimeoutMs = 10000
	}
	doc := handle(request.Station, request.Action, request.Payload, request.TimeoutMs)
	data, err := marshal(doc.object())
	if err != nil {
		fmt.Fprintf(os.Stderr, "control socket: %v\n", err)
		return
	}
	_, _ = conn.Write(append(data, '\n'))
}

// callControl is the `call` command: one request over the control socket.
func callControl(path string, station *string, action string, payload map[string]any, timeoutMs int64) int {
	conn, err := net.Dial("unix", path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "call: no running server or client run on %s: %v\n", path, err)
		return 3
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Duration(timeoutMs)*time.Millisecond + 15*time.Second))
	request, _ := marshal(controlRequest{Station: station, Action: action, Payload: payload, TimeoutMs: timeoutMs})
	if _, err := conn.Write(append(request, '\n')); err != nil {
		fmt.Fprintf(os.Stderr, "call: %v\n", err)
		return 3
	}
	line, _ := bufio.NewReaderSize(conn, 1<<20).ReadBytes('\n')
	var doc map[string]any
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.UseNumber()
	if len(bytes.TrimSpace(line)) == 0 || dec.Decode(&doc) != nil {
		fmt.Fprintln(os.Stderr, "call: the running process did not answer")
		return 1
	}
	writeRaw(append(bytes.TrimSpace(line), '\n'))
	errCode := ""
	if e, ok := doc["error"].(map[string]any); ok {
		errCode, _ = e["code"].(string)
	}
	return exitCodeOf(errCode)
}
