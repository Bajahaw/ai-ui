package tools

import (
	"strings"
	"testing"
)

func TestValidateOfficeZip(t *testing.T) {
	tests := []struct {
		name    string
		parts   map[string]string
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid Minimal Document",
			parts: map[string]string{
				"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
				"word/document.xml": `<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"></w:document>`,
				"_rels/.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
			},
			wantErr: false,
		},
		{
			name: "XML Syntax Error",
			parts: map[string]string{
				"word/document.xml": `<?xml version="1.0"?><w:document><unclosed>`,
			},
			wantErr: true,
			errMsg:  "XML syntax error",
		},
		{
			name: "Missing Content Type Part",
			parts: map[string]string{
				"[Content_Types].xml": `<?xml version="1.0"?>
<Types><Override PartName="/missing.xml" ContentType="foo"/></Types>`,
			},
			wantErr: true,
			errMsg:  "reference error in '[Content_Types].xml'",
		},
		{
			name: "Missing Relationship Target",
			parts: map[string]string{
				"_rels/.rels": `<?xml version="1.0"?>
<Relationships><Relationship Id="r1" Target="missing.xml"/></Relationships>`,
			},
			wantErr: true,
			errMsg:  "reference error in '_rels/.rels'",
		},
		{
			name: "Subdirectory Relationship Resolution",
			parts: map[string]string{
				"word/_rels/document.xml.rels": `<?xml version="1.0"?>
<Relationships><Relationship Id="r1" Target="styles.xml"/></Relationships>`,
				"word/styles.xml": `<styles/>`,
			},
			wantErr: false,
		},
		{
			name: "Missing Subdirectory Relationship Target",
			parts: map[string]string{
				"word/_rels/document.xml.rels": `<?xml version="1.0"?>
<Relationships><Relationship Id="r1" Target="styles.xml"/></Relationships>`,
			},
			wantErr: true,
			errMsg:  "reference error in 'word/_rels/document.xml.rels'",
		},
		{
			name: "Ignores Non-XML External Links",
			parts: map[string]string{
				"_rels/.rels": `<?xml version="1.0"?>
<Relationships><Relationship Id="r1" Target="https://google.com"/></Relationships>`,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf, err := buildZip(tt.parts)
			if err != nil {
				t.Fatalf("Failed to build zip for test: %v", err)
			}

			err = validateOfficeZip(buf.Bytes())
			if (err != nil) != tt.wantErr {
				t.Errorf("validateOfficeZip() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("validateOfficeZip() error = %v, want contained %v", err, tt.errMsg)
				}
			}
		})
	}
}

func TestValidateOfficeZip_NotAZip(t *testing.T) {
	err := validateOfficeZip([]byte("not a zip file"))
	if err == nil {
		t.Error("Expected error for invalid zip archive, got nil")
	} else if !strings.Contains(err.Error(), "invalid zip archive") {
		t.Errorf("Expected 'invalid zip archive' error, got: %v", err)
	}
}
