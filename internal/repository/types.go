package repository

type Type string

const (
	InMemory Type = "in-memory"
	MongoDB  Type = "mongodb"
)
