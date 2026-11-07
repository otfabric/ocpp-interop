// SPDX-License-Identifier: MIT
package main

// seanlee0923/ocpp's calls (csms.Call, station.Call) and handlers
// (csms.Handle, csms.HandleSend, station.Handle) are generic over its
// generated payload types. actionOps holds them for one action, made by
// callOps or sendOps (actions_gen.go has one per action and version), so
// that the adapter can call and answer an action named at run time with
// JSON. Every request and answer goes through the library's own types and
// schema validation.

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/seanlee0923/ocpp/csms"
	"github.com/seanlee0923/ocpp/protocol"
	"github.com/seanlee0923/ocpp/station"
)

// answerFunc answers a CALL: the request as JSON, the answer as JSON or an
// *answerError (a CALLERROR with that code).
type answerFunc func(ctx context.Context, request json.RawMessage) (json.RawMessage, error)

// sendFunc takes a SEND.
type sendFunc func(ctx context.Context, request json.RawMessage)

// answerError is the CALLERROR an answerFunc asks for.
type answerError struct{ code, description string }

func (e *answerError) Error() string { return e.code + ": " + e.description }

// requestError: the library refused the request before sending it.
type requestError struct{ err error }

func (e *requestError) Error() string { return e.err.Error() }
func (e *requestError) Unwrap() error { return e.err }

// responseError: the answer was not a valid confirmation of the action.
type responseError struct{ err error }

func (e *responseError) Error() string { return e.err.Error() }
func (e *responseError) Unwrap() error { return e.err }

type actionOps struct {
	check func(request json.RawMessage) error // the checks Call makes before sending
	send  bool                                // a SEND-only action (OCPP 2.1 NotifyPeriodicEventStream): no confirmation type

	csmsCall      func(ctx context.Context, s *csms.Session, request json.RawMessage) (json.RawMessage, error)
	stationCall   func(ctx context.Context, s *station.Station, request json.RawMessage) (json.RawMessage, error)
	csmsHandle    func(r *csms.Router, answer answerFunc) error
	stationHandle func(s *station.Station, answer answerFunc) error
	csmsHandleSnd func(r *csms.Router, take sendFunc) error
}

// decodeRequest is the request in the action's type, after the schema check
// the library makes of what it receives (ValidateJSON).
func decodeRequest[Q protocol.Payload](raw json.RawMessage) (Q, error) {
	var q Q
	if err := q.ValidateJSON(raw); err != nil {
		return q, &requestError{err}
	}
	if err := json.Unmarshal(raw, &q); err != nil {
		return q, &requestError{err}
	}
	return q, nil
}

// callResult classifies what csms.Call and station.Call return: a
// confirmation they could not validate or decode is a responseError.
func callResult[C protocol.Payload](c C, err error) (json.RawMessage, error) {
	if err != nil {
		var cr *csms.RemoteCallError
		var sr *station.RemoteCallError
		switch {
		case errors.As(err, &cr), errors.As(err, &sr):
			return nil, err
		case protocol.IsValidationError(err):
			// The request was validated before it was sent: this is the
			// confirmation's validation.
			return nil, &responseError{err}
		}
		var se *json.SyntaxError
		var te *json.UnmarshalTypeError
		if errors.As(err, &se) || errors.As(err, &te) {
			return nil, &responseError{err}
		}
		return nil, err
	}
	return json.Marshal(c)
}

func callOps[Q, C protocol.Payload]() actionOps {
	return actionOps{
		check: func(raw json.RawMessage) error {
			q, err := decodeRequest[Q](raw)
			if err != nil {
				return err
			}
			return q.Validate()
		},
		csmsCall: func(ctx context.Context, s *csms.Session, raw json.RawMessage) (json.RawMessage, error) {
			q, err := decodeRequest[Q](raw)
			if err != nil {
				return nil, err
			}
			return callResult(csms.Call[Q, C](ctx, s, q))
		},
		stationCall: func(ctx context.Context, s *station.Station, raw json.RawMessage) (json.RawMessage, error) {
			q, err := decodeRequest[Q](raw)
			if err != nil {
				return nil, err
			}
			return callResult(station.Call[Q, C](ctx, s, q))
		},
		csmsHandle: func(r *csms.Router, answer answerFunc) error {
			return csms.Handle[Q, C](r, func(ctx context.Context, _ *csms.Session, q Q) (C, error) {
				var zero C
				c, err := typedAnswer[Q, C](ctx, q, answer)
				if err != nil {
					var ae *answerError
					if errors.As(err, &ae) {
						return zero, &csms.CallError{Code: csms.ErrorCode(ae.code), Description: ae.description,
							Details: map[string]any{}}
					}
					return zero, err
				}
				return c, nil
			})
		},
		stationHandle: func(s *station.Station, answer answerFunc) error {
			return station.Handle[Q, C](s, func(ctx context.Context, q Q) (C, error) {
				var zero C
				c, err := typedAnswer[Q, C](ctx, q, answer)
				if err != nil {
					var ae *answerError
					if errors.As(err, &ae) {
						return zero, &station.CallError{Code: ae.code, Description: ae.description,
							Details: json.RawMessage(`{}`)}
					}
					return zero, err
				}
				return c, nil
			})
		},
	}
}

func sendOps[Q protocol.Payload]() actionOps {
	return actionOps{
		send: true,
		csmsHandleSnd: func(r *csms.Router, take sendFunc) error {
			return csms.HandleSend[Q](r, func(ctx context.Context, _ *csms.Session, q Q) error {
				raw, err := json.Marshal(q)
				if err != nil {
					return err
				}
				take(ctx, raw)
				return nil
			})
		},
	}
}

// typedAnswer gives the decoded request to answer as JSON and decodes its
// answer into the confirmation type; the library validates it before
// sending.
func typedAnswer[Q, C protocol.Payload](ctx context.Context, q Q, answer answerFunc) (C, error) {
	var c C
	raw, err := json.Marshal(q)
	if err != nil {
		return c, err
	}
	out, err := answer(ctx, raw)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(out, &c); err != nil {
		return c, &answerError{code: "InternalError", description: "the fixture answer is not a " +
			q.ActionName() + " confirmation: " + err.Error()}
	}
	return c, nil
}
