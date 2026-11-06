// SPDX-License-Identifier: MIT
package main

import (
	"encoding/json"

	"github.com/otfabric/go-ocpp"
	"github.com/otfabric/go-ocpp/ocpp16"
	"github.com/otfabric/go-ocpp/ocpp201"
	"github.com/otfabric/go-ocpp/ocpp21"
)

// versionInfo is what the adapter needs of one OCPP version, all of it from
// go-ocpp: the version, which role sends which action (the Actions table of
// the version package; DataTransfer goes both ways), which actions are a SEND
// (OCPP 2.1), and the package's own decoding and validation of a request and
// a response.
type versionInfo struct {
	version               ocpp.Version
	fromStation, fromCSMS map[string]bool
	sendActions           map[string]bool
	validateRequest       func(string, json.RawMessage) error
	validateResponse      func(string, json.RawMessage) error
}

// action is the part of the Action types of ocpp16, ocpp201 and ocpp21 the
// adapter reads.
type action struct {
	name string
	from ocpp.Role
	send bool
}

func newInfo(v ocpp.Version, actions []action, req, resp func(string, json.RawMessage) error) versionInfo {
	vi := versionInfo{version: v, fromStation: map[string]bool{}, fromCSMS: map[string]bool{},
		sendActions: map[string]bool{}, validateRequest: req, validateResponse: resp}
	for _, a := range actions {
		if a.from == 0 || a.from == ocpp.ChargingStation {
			vi.fromStation[a.name] = true
		}
		if a.from == 0 || a.from == ocpp.CSMS {
			vi.fromCSMS[a.name] = true
		}
		if a.send {
			vi.sendActions[a.name] = true
		}
	}
	return vi
}

func actions16() []action {
	out := make([]action, 0, len(ocpp16.Actions))
	for _, a := range ocpp16.Actions {
		out = append(out, action{a.Name, a.From, a.Send})
	}
	return out
}

func actions201() []action {
	out := make([]action, 0, len(ocpp201.Actions))
	for _, a := range ocpp201.Actions {
		out = append(out, action{a.Name, a.From, a.Send})
	}
	return out
}

func actions21() []action {
	out := make([]action, 0, len(ocpp21.Actions))
	for _, a := range ocpp21.Actions {
		out = append(out, action{a.Name, a.From, a.Send})
	}
	return out
}

var versions = map[string]versionInfo{
	"1.6":   newInfo(ocpp.V16, actions16(), ocpp16.ValidateRequest, ocpp16.ValidateResponse),
	"2.0.1": newInfo(ocpp.V201, actions201(), ocpp201.ValidateRequest, ocpp201.ValidateResponse),
	"2.1":   newInfo(ocpp.V21, actions21(), ocpp21.ValidateRequest, ocpp21.ValidateResponse),
}

// knows reports whether go-ocpp has the action in the version.
func knows(version, name string) bool {
	vi := versions[version]
	return vi.fromStation[name] || vi.fromCSMS[name]
}
