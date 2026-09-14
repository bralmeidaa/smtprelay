// Command smtprelay is the SMTP relay entrypoint. All logic lives in the
// relay package so it can be exercised by the integration tests under
// /tests without importing a "main" package (which Go disallows).
package main

import (
	"os"

	"github.com/bralmeidaa/smtprelay/src/relay"
)

func main() {
	os.Exit(relay.Run())
}
