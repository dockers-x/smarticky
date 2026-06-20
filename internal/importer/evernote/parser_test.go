package evernote

import (
	"strings"
	"testing"
)

func TestParseENEXNoteWithTagsAndResource(t *testing.T) {
	input := strings.NewReader(`<?xml version="1.0" encoding="UTF-8"?>
<en-export>
  <note>
    <title>Meeting</title>
    <content><![CDATA[<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE en-note SYSTEM "http://xml.evernote.com/pub/enml2.dtd"><en-note>Hello<img src="evernote:///view/1/s1/guid/res-guid/"/></en-note>]]></content>
    <created>20250101T010203Z</created>
    <updated>20250102T030405Z</updated>
    <tag>work</tag>
    <resource>
      <data encoding="base64">aGVsbG8=</data>
      <mime>text/plain</mime>
      <resource-attributes><file-name>hello.txt</file-name></resource-attributes>
    </resource>
  </note>
</en-export>`)
	doc, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(doc.Notes))
	}
	if doc.Notes[0].Title != "Meeting" {
		t.Fatalf("expected title Meeting, got %q", doc.Notes[0].Title)
	}
	if doc.Notes[0].Tags[0] != "work" {
		t.Fatalf("expected tag work, got %q", doc.Notes[0].Tags[0])
	}
	if string(doc.Notes[0].Resources[0].Data) != "hello" {
		t.Fatalf("expected decoded resource")
	}
}

func TestParseENEXKeepsInvalidResourceWithDecodeError(t *testing.T) {
	input := strings.NewReader(`<?xml version="1.0" encoding="UTF-8"?>
<en-export>
  <note>
    <title>Broken resource</title>
    <content><![CDATA[<en-note>Body</en-note>]]></content>
    <resource>
      <data encoding="base64">not-valid-base64</data>
      <mime>text/plain</mime>
    </resource>
  </note>
</en-export>`)

	doc, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(doc.Notes))
	}
	if len(doc.Notes[0].Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(doc.Notes[0].Resources))
	}
	if doc.Notes[0].Resources[0].DecodeError == "" {
		t.Fatalf("expected decode error")
	}
}
