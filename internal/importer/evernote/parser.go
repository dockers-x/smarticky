package evernote

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"
)

const evernoteTimeLayout = "20060102T150405Z"

type Document struct {
	Notes []Note
}

type Note struct {
	Title     string
	Content   string
	Created   time.Time
	Updated   time.Time
	Tags      []string
	Resources []Resource
}

type Resource struct {
	Data        []byte
	MIME        string
	FileName    string
	DecodeError string
}

type enExport struct {
	Notes []enNote `xml:"note"`
}

type enNote struct {
	Title     string       `xml:"title"`
	Content   string       `xml:"content"`
	Created   string       `xml:"created"`
	Updated   string       `xml:"updated"`
	Tags      []string     `xml:"tag"`
	Resources []enResource `xml:"resource"`
}

type enResource struct {
	Data       enResourceData       `xml:"data"`
	MIME       string               `xml:"mime"`
	Attributes enResourceAttributes `xml:"resource-attributes"`
}

type enResourceData struct {
	Encoding string `xml:"encoding,attr"`
	Value    string `xml:",chardata"`
}

type enResourceAttributes struct {
	FileName string `xml:"file-name"`
}

func Parse(r io.Reader) (*Document, error) {
	var raw enExport
	decoder := xml.NewDecoder(r)
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse enex: %w", err)
	}

	doc := &Document{Notes: make([]Note, 0, len(raw.Notes))}
	for _, rawNote := range raw.Notes {
		note, err := convertNote(rawNote)
		if err != nil {
			return nil, err
		}
		doc.Notes = append(doc.Notes, note)
	}

	return doc, nil
}

func convertNote(raw enNote) (Note, error) {
	created, err := parseEvernoteTime(raw.Created)
	if err != nil {
		return Note{}, fmt.Errorf("parse created time: %w", err)
	}
	updated, err := parseEvernoteTime(raw.Updated)
	if err != nil {
		return Note{}, fmt.Errorf("parse updated time: %w", err)
	}

	resources := make([]Resource, 0, len(raw.Resources))
	for _, rawResource := range raw.Resources {
		resources = append(resources, convertResource(rawResource))
	}

	return Note{
		Title:     strings.TrimSpace(raw.Title),
		Content:   raw.Content,
		Created:   created,
		Updated:   updated,
		Tags:      cleanTags(raw.Tags),
		Resources: resources,
	}, nil
}

func parseEvernoteTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	return time.Parse(evernoteTimeLayout, value)
}

func cleanTags(tags []string) []string {
	cleaned := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			cleaned = append(cleaned, tag)
		}
	}
	return cleaned
}

func convertResource(raw enResource) Resource {
	resource := Resource{
		MIME:     strings.TrimSpace(raw.MIME),
		FileName: strings.TrimSpace(raw.Attributes.FileName),
	}

	encoded := compactBase64(raw.Data.Value)
	if encoded == "" {
		return resource
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		resource.DecodeError = err.Error()
		return resource
	}
	resource.Data = data
	return resource
}

func compactBase64(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for _, r := range value {
		if !unicode.IsSpace(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}
