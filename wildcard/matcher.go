/*-
 * Copyright 2018 Square Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package wildcard implements simple wildcard matching meant to be used to
// match URIs and paths against simple patterns. It's less powerful but also
// less error-prone than regular expressions.
//
// We expose functions to build matchers from simple wildcard patterns. Each
// pattern is a sequence of segments separated by a separator, usually a
// forward slash. Each segment in the pattern may be a literal string, or a
// wildcard. A literal string will be matched exactly, a wildcard will match an
// arbitrary string.
//
// Two types of wildcards are supported:
//
// (1) A single '*' wildcard will match any literal string that does not
// contain the separator. It may occur anywhere between two separators in the
// pattern.
//
// (2) A double '**' wildcard will match anything, including the separator
// rune. It may only occur at the end of a pattern, after a separator.
//
// Furthermore, the matcher will consider the separator optional if it occurs
// at the end of a string. This means that, for example, the strings
// "test://foo/bar" and "test://foo/bar/" are treated as equivalent.
package wildcard

import (
	"errors"
	"fmt"
	"strings"
)

const (
	defaultSeparator = '/'
)

const (
	DEBUG = false
)

var (
	errEmptyPattern          = errors.New("input pattern was empty string")
	errInvalidWildcard       = errors.New("wildcard '*' can only appear between two separators")
	errInvalidDoubleWildcard = errors.New("wildcard '**' can only appear at end of pattern")
	errRegexpCompile         = errors.New("unable to compile generated regex (internal bug)")
	errInvalidPrefix         = errors.New("SPIFFE prefix invalid)")
	errInvalidSegment        = errors.New("Invalid SPIFFE segment (empty)")
)

// Matcher represents a compiled pattern that can be matched against a string.
type Matcher interface {
	// Matches checks if the given input matches the compiled pattern.
	Matches(string) bool
	GetSegments() []string
}

type splitMatcher struct {
	segments []string
}

// Compile creates a new Matcher given a pattern, using '/' as the separator.
func Compile(pattern string) (Matcher, error) {
	return CompileWithSeparator(pattern, defaultSeparator)
}

// CompileList creates new Matchers given a list patterns, using '/' as the separator.
func CompileList(patterns []string) ([]Matcher, error) {
	ms := []Matcher{}
	for _, pattern := range patterns {
		m, err := Compile(pattern)
		if err != nil {
			return nil, err
		}
		ms = append(ms, m)
	}
	return ms, nil
}

// MustCompile creates a new Matcher given a pattern, using '/' as the separator,
// and panics if the given pattern was invalid.
func MustCompile(pattern string) Matcher {
	if !PrefixCheck(pattern) {
		panic("Wrong prefix")
	}
	if InnerDoubleStar(pattern) {
		panic("Double star which is not at the end of pattern")
	}
	SuffixCheck(pattern)

	m, err := CompileWithSeparator(pattern, defaultSeparator)
	if err != nil {
		panic(err)
	}
	return m
}

func PrefixCheck(pattern string) bool {
	return strings.HasPrefix(pattern, "spiffe://")
}

func InnerDoubleStar(pattern string) bool {
	firstinstance := strings.Index(pattern, "**")
	return firstinstance > -1 && firstinstance < len(pattern)-2
}

func SuffixCheck(pattern string) {
	if strings.HasSuffix(pattern, "/") {
		// TODO: Warn
	}
}

// CompileWithSeparator creates a new Matcher given a pattern and separator rune.
func CompileWithSeparator(pattern string, separator rune) (Matcher, error) {

	if pattern == "" {
		return nil, errEmptyPattern
	}

	if !PrefixCheck(pattern) {
		return nil, errInvalidPrefix
	}

	if InnerDoubleStar(pattern) {
		return nil, errInvalidDoubleWildcard
	}

	segments := GetSegmentsFromURI(pattern, defaultSeparator)
	// Check for malformed URI
	for i, _ := range segments {
		// "**" Embedded in a segment
		if len(segments[i]) > 2 && strings.Contains(segments[i], "**") {
			return nil, errInvalidDoubleWildcard
			// "*" Embedded in a segment - other than "**"
		} else if len(segments[i]) > 1 && segments[i] != "**" && strings.Contains(segments[i], "*") {
			return nil, errInvalidWildcard
		}
		// Empty segment, e.g.: "//"
		if len(segments[i]) == 0 {
			return nil, errInvalidSegment
		}
	}

	return splitMatcher{
		segments: segments,
	}, nil
}

func ParseURIWithSeparator(uri string, separator rune) (Matcher, error) {

	if uri == "" {
		return nil, errEmptyPattern
	}

	if !PrefixCheck(uri) {
		return nil, errInvalidPrefix
	}

	segments := GetSegmentsFromURI(uri, defaultSeparator)
	// Check for malformed URI
	for i, _ := range segments {
		// Empty segment, e.g.: "//"
		if len(segments[i]) == 0 {
			return nil, errInvalidSegment
		}
	}

	return splitMatcher{
		segments: segments,
	}, nil
}

// This function assumes the URI is well-formed. Specifically that it starts with "spiffe://".
// Otherwise could violate bounds
func GetSegmentsFromURI(acl string, separator rune) []string {
	// For trailing slash
	var segments []string
	if acl[len(acl)-1] == '/' {
		segments = strings.Split(string(acl[9:len(acl)-1]), string(separator))
	} else {
		// Default case
		segments = strings.Split(string(acl[9:]), string(separator))
	}
	return segments
}

// Matches checks if the given input matches the compiled pattern.
func (acl splitMatcher) Matches(input string) bool {
	//return rm.pattern.Match([]byte(input))
	uriSegments, err := ParseURIWithSeparator(input, defaultSeparator)

	if err != nil {
		return false
	}

	if DEBUG {
		fmt.Println("Comparing: ", strings.Join(uriSegments.GetSegments(), "!"))
		fmt.Println("	Length: ", len(uriSegments.GetSegments()))
		fmt.Println("With ACL : ", strings.Join(acl.segments, "!"))
		fmt.Println("	Length: ", len(acl.segments))
	}

	minlen := len(uriSegments.GetSegments())
	if len(acl.segments) < minlen {
		minlen = len(acl.segments)
	}

	for i := 0; i < minlen; i++ {
		if DEBUG {
			fmt.Println("ACL segment: ", acl.segments[i])
			fmt.Println("URI segment: ", uriSegments.GetSegments()[i])
			fmt.Println("")
		}
		// Current segment matches
		if acl.segments[i] == "*" || acl.segments[i] == uriSegments.GetSegments()[i] {
			if DEBUG {
				fmt.Println("[+] continue")
			}
			continue
			// "**" means we are done and the match was successful
		} else if acl.segments[i] == "**" {
			if DEBUG {
				fmt.Println("[+] ** true - end")
			}
			return true
		} else {
			if DEBUG {
				fmt.Println("[+] false - end")
			}
			return false
		}
	}

	// Standard case: End reached without conflicts
	if len(uriSegments.GetSegments()) == len(acl.segments) {
		return true
	}

	// Special case: "**" after the URI is done.
	// This must also be the last segment of the ACL.
	// We assume the ACL to be properly formatted here
	// And don't need to check this
	if len(acl.segments) > minlen && acl.segments[minlen] == "**" {
		return true
	}

	// If none of the above have worked, URI and ACL don't match
	return false

}

func (acl splitMatcher) GetSegments() []string {
	return acl.segments
}
