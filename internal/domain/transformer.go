package domain

import "io"

// Transformer defines the interface for parsing and transforming raw data into Articles.
type Transformer interface {
	Transform(reader io.Reader) ([]Article, error)
}
