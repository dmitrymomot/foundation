package main

import "foundation-basic-example/db/repository"

type SessionData struct{}

type sessionStorage struct {
	repo repository.Querier
}
