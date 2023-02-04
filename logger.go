package ghinstallation

import (
	"fmt"
	"net/http"
)

type Logger interface {
	Printf(string, ...interface{})
}

type LeveledLogger interface {
	Infow(msg string, keysAndValues ...interface{})
	Debugw(msg string, keysAndValues ...interface{})
}

func respKVs(resp *http.Response) []interface{} {

	if resp == nil {
		return []interface{}{"response", resp}
	}
	var hdrs []string
	for k, vals := range resp.Header {
		for _, v := range vals {
			hdrs = append(hdrs, fmt.Sprintf("%v: %v", k, v))
		}
	}
	kvs := []interface{}{
		"response.Status", resp.Status,
		"response.Header", hdrs,
	}
	if req := resp.Request; req != nil {
		kvs = append(kvs,
			"response.Request.Method", req.Method,
			"response.Request.URL", req.URL.String(),
		)

	}

	return kvs
}
