package testutil

import (
	"net/http"
)

type roundTrip struct {
	theResponse *http.Response
	theError    error
}

func (rt *roundTrip) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt.theResponse, rt.theError
}

type roundTripCheckRequest struct {
	checkRequest func(*http.Request)
	roundTrip
}

func (rt *roundTripCheckRequest) RoundTrip(r *http.Request) (*http.Response, error) {
	rt.checkRequest(r)
	return rt.theResponse, rt.theError
}

// NewRoundTripCheckRequest creates a new instance of a type that implements http.RoundTripper,
// which before returning theResponse and theError, executes checkRequest against a http.Request.
func NewRoundTripCheckRequest(checkRequest func(*http.Request), theResponse *http.Response, theError error) http.RoundTripper {
	return &roundTripCheckRequest{
		checkRequest: checkRequest,
		roundTrip: roundTrip{
			theResponse: theResponse,
			theError:    theError}}
}
