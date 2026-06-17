package tools

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

// ── Test helper ─────────────────────────────────────────────────────────

// buildTestZip creates a zip archive from a map of path→content.
func buildTestZip(t *testing.T, parts map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range parts {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry %q: %v", name, err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write zip entry %q: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close zip: %v", err)
	}
	return buf.Bytes()
}

// xlsxBase returns the standard xlsx skeleton parts.
// Callers can override individual parts before passing to buildTestZip.
func xlsxBase() map[string]string {
	return map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
  <Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/>
</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/>
</Relationships>`,
		"xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"
          xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets>
</workbook>`,
		"xl/styles.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <fonts count="1"><font><sz val="11"/><name val="Calibri"/></font></fonts>
  <fills count="2">
    <fill><patternFill patternType="none"/></fill>
    <fill><patternFill patternType="gray125"/></fill>
  </fills>
  <borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>
  <cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
  <cellXfs count="2">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0" applyFont="1"/>
  </cellXfs>
</styleSheet>`,
		"xl/sharedStrings.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="3" uniqueCount="3">
  <si><t>Hello</t></si>
  <si><t>World</t></si>
  <si><t>Test</t></si>
</sst>`,
		"xl/worksheets/sheet1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1"><c r="A1" t="s" s="0"><v>0</v></c><c r="B1" t="s" s="1"><v>1</v></c></row>
    <row r="2"><c r="A2" t="s" s="0"><v>2</v></c></row>
  </sheetData>
</worksheet>`,
	}
}

// ── Valid cases ──────────────────────────────────────────────────────────

func TestValidMinimalXLSX(t *testing.T) {
	parts := xlsxBase()
	// Use empty sheet with no data
	parts["xl/sharedStrings.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="0" uniqueCount="0"/>`
	parts["xl/worksheets/sheet1.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData/></worksheet>`
	parts["xl/styles.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <fonts count="1"><font><sz val="11"/><name val="Calibri"/></font></fonts>
  <fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills>
  <borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>
  <cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
  <cellXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/></cellXfs>
</styleSheet>`

	data := buildTestZip(t, parts)
	if err := validateOfficeZip(data); err != nil {
		t.Errorf("valid minimal xlsx should pass, got: %v", err)
	}
}

func TestValidXLSXWithData(t *testing.T) {
	data := buildTestZip(t, xlsxBase())
	if err := validateOfficeZip(data); err != nil {
		t.Errorf("valid xlsx with data should pass, got: %v", err)
	}
}

func TestValidDocx(t *testing.T) {
	data := buildTestZip(t, minimalTemplates["docx"])
	if err := validateOfficeZip(data); err != nil {
		t.Errorf("valid docx should pass, got: %v", err)
	}
}

func TestValidPptx(t *testing.T) {
	data := buildTestZip(t, minimalTemplates["pptx"])
	if err := validateOfficeZip(data); err != nil {
		t.Errorf("valid pptx should pass, got: %v", err)
	}
}

// ── Shared string errors ────────────────────────────────────────────────

func TestSharedStringIndexOutOfBounds(t *testing.T) {
	parts := xlsxBase()
	// Only 3 shared strings (indices 0,1,2) but cell references index 5
	parts["xl/worksheets/sheet1.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1"><c r="A1" t="s"><v>5</v></c></row>
  </sheetData>
</worksheet>`

	data := buildTestZip(t, parts)
	err := validateOfficeZip(data)
	if err == nil {
		t.Fatal("expected error for shared string index out of bounds, got nil")
	}
	if !strings.Contains(err.Error(), "shared string error") {
		t.Errorf("expected shared string error, got: %v", err)
	}
}

func TestSharedStringUniqueCountMismatch(t *testing.T) {
	parts := xlsxBase()
	// Declares uniqueCount=10 but only has 3 entries
	parts["xl/sharedStrings.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="10" uniqueCount="10">
  <si><t>Hello</t></si>
  <si><t>World</t></si>
  <si><t>Test</t></si>
</sst>`
	// Remove cell references so we only test the count mismatch
	parts["xl/worksheets/sheet1.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData/></worksheet>`

	data := buildTestZip(t, parts)
	err := validateOfficeZip(data)
	if err == nil {
		t.Fatal("expected error for uniqueCount mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "uniqueCount=10") && !strings.Contains(err.Error(), "3 entries") {
		t.Errorf("expected uniqueCount mismatch error, got: %v", err)
	}
}

// ── Style errors ────────────────────────────────────────────────────────

func TestStyleIndexOutOfBounds(t *testing.T) {
	parts := xlsxBase()
	// styles.xml has 2 cellXf entries (indices 0,1) but cell references s="9"
	parts["xl/worksheets/sheet1.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1"><c r="A1" t="s" s="9"><v>0</v></c></row>
  </sheetData>
</worksheet>`

	data := buildTestZip(t, parts)
	err := validateOfficeZip(data)
	if err == nil {
		t.Fatal("expected error for style index out of bounds, got nil")
	}
	if !strings.Contains(err.Error(), "style error") {
		t.Errorf("expected style error, got: %v", err)
	}
}

func TestCellXfsCountMismatch(t *testing.T) {
	parts := xlsxBase()
	// Declares count=5 but only has 2 xf entries
	parts["xl/styles.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <fonts count="1"><font><sz val="11"/><name val="Calibri"/></font></fonts>
  <fills count="2">
    <fill><patternFill patternType="none"/></fill>
    <fill><patternFill patternType="gray125"/></fill>
  </fills>
  <borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>
  <cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
  <cellXfs count="5">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0" applyFont="1"/>
  </cellXfs>
</styleSheet>`

	data := buildTestZip(t, parts)
	err := validateOfficeZip(data)
	if err == nil {
		t.Fatal("expected error for cellXfs count mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "cellXfs count=5") {
		t.Errorf("expected cellXfs count mismatch error, got: %v", err)
	}
}

// ── Row ordering ────────────────────────────────────────────────────────

func TestRowOrderingViolation(t *testing.T) {
	parts := xlsxBase()
	// Row 3 appears before row 2
	parts["xl/worksheets/sheet1.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1"><c r="A1" t="s"><v>0</v></c></row>
    <row r="3"><c r="A3" t="s"><v>1</v></c></row>
    <row r="2"><c r="A2" t="s"><v>2</v></c></row>
  </sheetData>
</worksheet>`

	data := buildTestZip(t, parts)
	err := validateOfficeZip(data)
	if err == nil {
		t.Fatal("expected error for row ordering violation, got nil")
	}
	if !strings.Contains(err.Error(), "row ordering error") {
		t.Errorf("expected row ordering error, got: %v", err)
	}
}

func TestDuplicateRowNumbers(t *testing.T) {
	parts := xlsxBase()
	// Two rows both claiming to be row 1
	parts["xl/worksheets/sheet1.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1"><c r="A1" t="s"><v>0</v></c></row>
    <row r="1"><c r="B1" t="s"><v>1</v></c></row>
  </sheetData>
</worksheet>`

	data := buildTestZip(t, parts)
	err := validateOfficeZip(data)
	if err == nil {
		t.Fatal("expected error for duplicate row numbers, got nil")
	}
	if !strings.Contains(err.Error(), "row ordering error") {
		t.Errorf("expected row ordering error, got: %v", err)
	}
}

// ── XML syntax ──────────────────────────────────────────────────────────

func TestMalformedXML(t *testing.T) {
	parts := xlsxBase()
	// Unclosed <c> element — the exact error from the model.txt log
	parts["xl/worksheets/sheet1.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></row>
  </sheetData>
</worksheet>`

	data := buildTestZip(t, parts)
	err := validateOfficeZip(data)
	if err == nil {
		t.Fatal("expected error for malformed XML, got nil")
	}
	if !strings.Contains(err.Error(), "XML syntax error") {
		t.Errorf("expected XML syntax error, got: %v", err)
	}
}

// ── Content type references ─────────────────────────────────────────────

func TestMissingContentTypeReference(t *testing.T) {
	parts := xlsxBase()
	// Content Types references a part that doesn't exist
	parts["[Content_Types].xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
  <Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/>
  <Override PartName="/xl/ghost.xml" ContentType="application/xml"/>
</Types>`

	data := buildTestZip(t, parts)
	err := validateOfficeZip(data)
	if err == nil {
		t.Fatal("expected error for missing content type reference, got nil")
	}
	if !strings.Contains(err.Error(), "ghost.xml") {
		t.Errorf("expected error mentioning ghost.xml, got: %v", err)
	}
}

// ── Relationship references ─────────────────────────────────────────────

func TestMissingRelationshipTarget(t *testing.T) {
	parts := xlsxBase()
	// workbook.xml.rels points to a sheet that doesn't exist
	parts["xl/_rels/workbook.xml.rels"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/>
  <Relationship Id="rId4" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet99.xml"/>
</Relationships>`

	data := buildTestZip(t, parts)
	err := validateOfficeZip(data)
	if err == nil {
		t.Fatal("expected error for missing relationship target, got nil")
	}
	if !strings.Contains(err.Error(), "sheet99.xml") {
		t.Errorf("expected error mentioning sheet99.xml, got: %v", err)
	}
}

// ── Edge cases ──────────────────────────────────────────────────────────

func TestSharedStringBoundaryIndex(t *testing.T) {
	parts := xlsxBase()
	// 3 shared strings, reference index 2 (last valid) — should pass
	parts["xl/worksheets/sheet1.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1"><c r="A1" t="s"><v>2</v></c></row>
  </sheetData>
</worksheet>`

	data := buildTestZip(t, parts)
	if err := validateOfficeZip(data); err != nil {
		t.Errorf("referencing last valid shared string index should pass, got: %v", err)
	}

	// Now reference index 3 (one past the end) — should fail
	parts["xl/worksheets/sheet1.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1"><c r="A1" t="s"><v>3</v></c></row>
  </sheetData>
</worksheet>`

	data = buildTestZip(t, parts)
	err := validateOfficeZip(data)
	if err == nil {
		t.Fatal("referencing index past end of shared strings should fail, got nil")
	}
}

func TestStyleBoundaryIndex(t *testing.T) {
	parts := xlsxBase()
	// 2 cellXf entries (indices 0,1), reference s="1" (last valid) — should pass
	parts["xl/worksheets/sheet1.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1"><c r="A1" t="s" s="1"><v>0</v></c></row>
  </sheetData>
</worksheet>`

	data := buildTestZip(t, parts)
	if err := validateOfficeZip(data); err != nil {
		t.Errorf("referencing last valid style index should pass, got: %v", err)
	}

	// Now reference s="2" (one past the end) — should fail
	parts["xl/worksheets/sheet1.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1"><c r="A1" t="s" s="2"><v>0</v></c></row>
  </sheetData>
</worksheet>`

	data = buildTestZip(t, parts)
	err := validateOfficeZip(data)
	if err == nil {
		t.Fatal("referencing style index past end should fail, got nil")
	}
}

func TestNonXLSXPartsIgnored(t *testing.T) {
	// Docx should not trigger xlsx semantic validation
	parts := minimalTemplates["docx"]
	data := buildTestZip(t, parts)
	if err := validateOfficeZip(data); err != nil {
		t.Errorf("docx should not trigger xlsx validation, got: %v", err)
	}
}
