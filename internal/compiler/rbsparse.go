package compiler

import "rb2go/internal/rbs"

func parseTypeString(s string) (rbs.Type, error) { return rbs.ParseType(s) }
