package domain

import "io"

// PageInfo contains pagination metadata from the response.
type PageInfo struct {
	Page       int `json:"page"`
	NumPages   int `json:"numPages"`
	PageSize   int `json:"pageSize"`
	NumEntries int `json:"numEntries"`
}

// Transformer defines the interface for parsing and transforming raw data into Articles.
type Transformer interface {
	Transform(reader io.Reader) ([]Article, *PageInfo, error)
}
