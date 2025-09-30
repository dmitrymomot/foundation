package queue

import (
	"fmt"
	"strings"
)

// qualifiedStructName extracts the type name from any value, removing pointer prefixes.
// Used to generate task names from payload types (e.g., "EmailPayload" from EmailPayload{}).
func qualifiedStructName(v any) string {
	s := fmt.Sprintf("%T", v)
	s = strings.TrimLeft(s, "*")

	return s
}
