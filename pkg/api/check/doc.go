// Package check provides the /v1/check HTTP API endpoint, reporting whether a
// newer image is available for each watched container without applying any update.
//
// For every container Watchtower watches it compares the running image's digest
// against the registry using a HEAD request (the same staleness check the update
// path uses to decide whether a pull is needed), so it pulls no layers and recreates
// no containers. This lets an external application show an "update available" banner
// using Watchtower's existing registry credentials, with no side effects.
//
// Key components:
//   - Handler: Serves the /v1/check endpoint with per-container update availability.
//   - Result: Data model for a single container's check outcome (name, image,
//     current and latest digests, update_available, and an optional per-container error).
//   - New: Creates a handler with a check function for fetching results.
//
// Usage example:
//
//	handler := check.New(checkFunc)
//	http.HandleFunc("GET "+handler.Path, handler.Handle)
//
// The package returns JSON responses with a results array, count, the time the
// check was performed, and the API version.
package check
