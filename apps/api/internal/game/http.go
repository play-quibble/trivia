package game

import (
	"encoding/json"
	"net/http"
)

// writeJSON serializes v to JSON and sends it as the HTTP response.
// The Content-Type header must be set before WriteHeader is called — in Go,
// once you call WriteHeader (or implicitly trigger it by writing the body),
// headers are locked and can't be changed.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Encode writes directly to the ResponseWriter (a stream), so there's no
	// intermediate string/buffer allocation. The blank identifier _ discards
	// the error because the header is already sent — we can't send a new error
	// response at this point anyway.
	_ = json.NewEncoder(w).Encode(v)
}

// readJSON decodes the JSON request body into v (which must be a pointer).
// defer r.Body.Close() ensures the body is closed after this function returns,
// regardless of whether decoding succeeds or fails — similar to a finally block
// in Java or a with-statement in Python.
func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// writeError sends a JSON response with a single "error" field.
// Keeping error responses in a consistent shape makes them easy to handle
// on the client side without special-casing.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
