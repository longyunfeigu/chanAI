package parser

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Parser defines how to parse the output of an LLM.
type Parser[T any] interface {
	// Parse converts the output text into a structured object.
	Parse(text string) (T, error)
	// GetFormatInstructions returns a string describing the expected format.
	GetFormatInstructions() string
}

// JSONParser parses JSON output into a struct.
type JSONParser[T any] struct {
	// Optional schema description
}

// NewJSONParser creates a new JSON parser.
func NewJSONParser[T any]() *JSONParser[T] {
	return &JSONParser[T]{}
}

// Parse tries to extract and parse JSON from the text.
// It handles cases where the JSON is embedded in markdown code blocks.
func (p *JSONParser[T]) Parse(text string) (T, error) {
	var zero T
	cleaned := cleanJSON(text)
	
	if err := json.Unmarshal([]byte(cleaned), &zero); err != nil {
		return zero, fmt.Errorf("failed to parse JSON: %w. Input: %s", err, text)
	}
	return zero, nil
}

func (p *JSONParser[T]) GetFormatInstructions() string {
	return "Return the output as a valid JSON object."
}

// StringParser returns the raw text.
type StringParser struct{}

func NewStringParser() *StringParser {
	return &StringParser{}
}

func (p *StringParser) Parse(text string) (string, error) {
	return text, nil
}

func (p *StringParser) GetFormatInstructions() string {
	return ""
}

// cleanJSON extracts JSON from markdown code blocks or strips surrounding whitespace.
func cleanJSON(text string) string {
	text = strings.TrimSpace(text)
	
	// Check for markdown code blocks
	re := regexp.MustCompile("(?s)```(?:json)?(.*?)```")
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	
	return text
}
