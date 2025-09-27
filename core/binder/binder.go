package binder

import "net/http"

type Binder func(r *http.Request, v any) error
