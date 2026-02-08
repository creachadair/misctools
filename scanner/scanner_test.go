// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

package scanner_test

import (
	"strings"
	"testing"

	"github.com/creachadair/misctools/scanner"
	"github.com/google/go-cmp/cmp"
)

func TestScanner(t *testing.T) {
	tests := []struct {
		input string
		want  []scanner.Token
	}{
		// Empty inputs
		{"", nil},
		{"  ", nil},
		{"\n\n  \n", nil},
		{"\t  \r\n \t  \r\n", nil},

		// Strings
		{`"" "a b c" "a\nb\tc"`, []scanner.Token{scanner.String, scanner.String, scanner.String}},
		{`"\"\\\/\b\f\n\r\t"`, []scanner.Token{scanner.String}},
		{`"\u0000\u01fc\uAA9c"`, []scanner.Token{scanner.String}},

		// Numbers
		{`0 -1 5139 23`, []scanner.Token{
			scanner.Integer, scanner.Integer, scanner.Integer, scanner.Integer,
		}},
	}

	for _, test := range tests {
		var got []scanner.Token
		s := scanner.New(strings.NewReader(test.input))
		for s.Next() {
			got = append(got, s.Token())
		}
		if s.Err() != nil {
			t.Errorf("Next failed: %v", s.Err())
		}
		if diff := cmp.Diff(test.want, got); diff != "" {
			t.Errorf("Input: %#q\nTokens: (-want, +got)\n%s", test.input, diff)
		}
	}
}

func TestScanner_decodeAs(t *testing.T) {
	mustScan := func(t *testing.T, input string, want scanner.Token) *scanner.Scanner {
		t.Helper()
		s := scanner.New(strings.NewReader(input))
		if !s.Next() {
			t.Fatalf("Next failed: %v", s.Err())
		} else if s.Token() != want {
			t.Fatalf("Next token: got %v, want %v", s.Token(), want)
		}
		return s
	}

	t.Run("Integer", func(t *testing.T) {
		mustScan(t, `-15`, scanner.Integer)
	})
	t.Run("String", func(t *testing.T) {
		const wantText = `"a\tb\u0020c\n"` // as written, without quotes
		s := mustScan(t, `"a\tb\u0020c\n"`, scanner.String)
		text := s.Text()
		if got := string(text); got != wantText {
			t.Errorf("Text: got %#q, want %#q", got, wantText)
		}
	})
}

func TestScannerLoc(t *testing.T) {
	type tokPos struct {
		Tok scanner.Token
		Pos string
	}
	tests := []struct {
		input string
		want  []tokPos
	}{
		{"", nil},
		{"0 1", []tokPos{{scanner.Integer, "1:0-1"}, {scanner.Integer, "1:2-3"}}},
		{`"foo"`, []tokPos{{scanner.String, "1:0-5"}}},
	}
	for _, tc := range tests {
		var got []tokPos
		s := scanner.New(strings.NewReader(tc.input))
		for s.Next() {
			got = append(got, tokPos{s.Token(), s.Location().String()})
		}
		if s.Err() != nil {
			t.Errorf("Next failed: %v", s.Err())
		}
		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Errorf("Input: %#q\nTokens: (-want, +got)\n%s", tc.input, diff)
		}
	}
}
