package cache

import (
	"fmt"
	"strings"
)

const endpointListPrefix = "mocksvr:endpoints"

// EndpointListKey returns the cache key holding the active endpoints for a
// method. It is shared by the dynamic router (reads) and the endpoint service
// (invalidates), which keeps both sides consistent.
func EndpointListKey(method string) string {
	return fmt.Sprintf("%s:%s", endpointListPrefix, strings.ToUpper(method))
}
