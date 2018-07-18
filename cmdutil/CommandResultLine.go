package cmdutil

import (
	"fmt"
	"regexp"
	"strings"
)

// CommandResultLine is one text line of stdout of command
type CommandResultLine struct {
	line   string
	Fields []*StringField
}

func newCommandResultLine(s string, trim bool, splitNumber int) *CommandResultLine {
	if trim {
		s = strings.TrimSpace(s)
	}

	l := &CommandResultLine{
		line: s,
	}

	r := regexp.MustCompile(`\s+`)
	// log.Printf("slpit(%d) %s\n", splitNumber, s)
	fields := r.Split(s, splitNumber)
	// log.Printf("split done, len is %d\n", len(fields))
	for _, t := range fields {
		// log.Printf("======= %s\n", t)
		l.Fields = append(l.Fields, newStringField(t))
	}

	return l
}

func (t *CommandResultLine) String() string {
	return t.line
}

// GetField TODO
func (t *CommandResultLine) GetField(idx int) *StringField {
	if idx < 0 || idx >= len(t.Fields) {
		return &StringField{
			err: fmt.Errorf("index out of range: idx=%d, len=%d", idx, len(t.Fields)),
		}
	}

	return t.Fields[idx]
}
