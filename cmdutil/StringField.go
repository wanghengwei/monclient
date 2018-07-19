package cmdutil

import (
	"fmt"
	"regexp"
	"strconv"
)

// StringField TODO
type StringField struct {
	content string
	err     error
}

func newStringField(s string) *StringField {
	return &StringField{
		content: s,
	}
}

func (s *StringField) String() string {
	return s.content
}

// AsUInt64 convert to uint64
func (s *StringField) AsUInt64() uint64 {
	if s.err != nil {
		return 0
	}

	r, err := strconv.ParseUint(s.content, 10, 64)
	if err != nil {
		return 0
	}

	return r
}

// AsFloat32 todo
func (s *StringField) AsFloat32() float32 {
	if s.err != nil {
		return 0
	}

	r, err := strconv.ParseFloat(s.content, 32)
	if err != nil {
		return 0
	}

	return float32(r)
}

// AsInt TODO
func (s *StringField) AsInt() int {
	if s.err != nil {
		return 0
	}

	r, err := strconv.Atoi(s.content)
	if err != nil {
		return 0
	}

	return r
}

// FindSubmatch TODO
func (s *StringField) FindSubmatch(pattern string) *StringField {
	if s.err != nil {
		return s
	}

	r, err := regexp.Compile(pattern)
	if err != nil {
		return &StringField{err: err}
	}

	ms := r.FindStringSubmatch(s.content)
	if len(ms) < 2 {
		return &StringField{err: fmt.Errorf("not found subgroup: %s in %s", pattern, s.content)}
	}

	return newStringField(ms[1])
}

func (s *StringField) FindSubmatches(pattern string) ([]string, error) {
	if s.err != nil {
		return nil, s.err
	}

	r, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	ms := r.FindStringSubmatch(s.content)
	return ms, nil
}
