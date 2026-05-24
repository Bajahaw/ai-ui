package tools

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	fs "github.com/Bajahaw/ai-ui/cmd/files"
	"github.com/Bajahaw/ai-ui/cmd/providers"
	"github.com/google/uuid"
)

// ── list_document_parts ─────────────────────────────────────────────────

func listDocumentPartsTool(args, user string) providers.ToolOutput {
	var params struct {
		FileID string `json:"file_id"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error decoding arguments: %v", err)}
	}

	filePath, err := resolveFilePath(params.FileID, user)
	if err != nil {
		return providers.ToolOutput{Content: err.Error()}
	}

	r, err := zip.OpenReader(filePath)
	if err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error opening document archive: %v", err)}
	}
	defer r.Close()

	type entry struct {
		Path string `json:"path"`
		Size uint64 `json:"size"`
		Type string `json:"type"`
	}

	entries := make([]entry, 0, len(r.File))
	for _, f := range r.File {
		kind := "binary"
		lower := strings.ToLower(f.Name)
		if strings.HasSuffix(lower, ".xml") || strings.HasSuffix(lower, ".rels") ||
			lower == "[content_types].xml" {
			kind = "xml"
		} else if strings.HasSuffix(lower, ".txt") || strings.HasSuffix(lower, ".csv") {
			kind = "text"
		}
		entries = append(entries, entry{
			Path: f.Name,
			Size: f.UncompressedSize64,
			Type: kind,
		})
	}

	out, _ := json.MarshalIndent(entries, "", "  ")
	return providers.ToolOutput{Content: string(out)}
}

// ── read_document_part ──────────────────────────────────────────────────

func readDocumentPartTool(args, user string) providers.ToolOutput {
	var params struct {
		FileID   string `json:"file_id"`
		PartPath string `json:"part_path"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error decoding arguments: %v", err)}
	}

	filePath, err := resolveFilePath(params.FileID, user)
	if err != nil {
		return providers.ToolOutput{Content: err.Error()}
	}

	r, err := zip.OpenReader(filePath)
	if err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error opening document archive: %v", err)}
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == params.PartPath {
			rc, err := f.Open()
			if err != nil {
				return providers.ToolOutput{Content: fmt.Sprintf("error reading part: %v", err)}
			}
			defer rc.Close()

			data, err := io.ReadAll(rc)
			if err != nil {
				return providers.ToolOutput{Content: fmt.Sprintf("error reading part content: %v", err)}
			}

			return providers.ToolOutput{Content: string(data)}
		}
	}

	return providers.ToolOutput{Content: fmt.Sprintf("part '%s' not found in document", params.PartPath)}
}

// ── create_document ─────────────────────────────────────────────────────

func createDocumentTool(args, user string) providers.ToolOutput {
	var params struct {
		FileName string            `json:"file_name"`
		Format   string            `json:"format"`
		Parts    map[string]string `json:"parts"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error decoding arguments: %v", err)}
	}

	if params.FileName == "" || params.Format == "" {
		return providers.ToolOutput{Content: "error: file_name and format are required"}
	}

	format := strings.ToLower(params.Format)

	// Start with minimal template for the format, then overlay user parts
	template, ok := minimalTemplates[format]
	if !ok {
		return providers.ToolOutput{Content: fmt.Sprintf("unsupported format '%s'; supported: docx, pptx, xlsx", params.Format)}
	}

	// Merge: template provides defaults, user parts override
	merged := make(map[string]string, len(template)+len(params.Parts))
	for k, v := range template {
		merged[k] = v
	}
	for k, v := range params.Parts {
		merged[k] = v
	}

	buf, err := buildZip(merged)
	if err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error creating document: %v", err)}
	}

	if err := validateOfficeZip(buf.Bytes()); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("Validation failed. File was NOT saved.\nError: %v", err)}
	}

	fileData, err := saveGeneratedFile(buf.Bytes(), params.FileName, user)
	if err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error saving document: %v", err)}
	}

	return providers.ToolOutput{
		Content: fmt.Sprintf("Created '%s' (%s, %d bytes). File ID: %s Path: %s", params.FileName, format, fileData.Size, fileData.ID, fileData.Path),
	}
}

// ── write_document_part ─────────────────────────────────────────────────

func writeDocumentPartTool(args, user string) providers.ToolOutput {
	var params struct {
		FileID   string `json:"file_id"`
		PartPath string `json:"part_path"`
		Content  string `json:"content"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error decoding arguments: %v", err)}
	}

	filePath, err := resolveFilePath(params.FileID, user)
	if err != nil {
		return providers.ToolOutput{Content: err.Error()}
	}

	r, err := zip.OpenReader(filePath)
	if err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error opening document archive: %v", err)}
	}
	defer r.Close()

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	replaced := false
	for _, f := range r.File {
		fw, err := w.Create(f.Name)
		if err != nil {
			return providers.ToolOutput{Content: fmt.Sprintf("error writing archive: %v", err)}
		}

		if f.Name == params.PartPath {
			// Replace with new content
			if _, err := fw.Write([]byte(params.Content)); err != nil {
				return providers.ToolOutput{Content: fmt.Sprintf("error writing part: %v", err)}
			}
			replaced = true
		} else {
			// Copy original
			rc, err := f.Open()
			if err != nil {
				return providers.ToolOutput{Content: fmt.Sprintf("error reading original part: %v", err)}
			}
			if _, err := io.Copy(fw, rc); err != nil {
				rc.Close()
				return providers.ToolOutput{Content: fmt.Sprintf("error copying part: %v", err)}
			}
			rc.Close()
		}
	}

	// If part didn't exist, add it as a new entry
	if !replaced {
		fw, err := w.Create(params.PartPath)
		if err != nil {
			return providers.ToolOutput{Content: fmt.Sprintf("error adding new part: %v", err)}
		}
		if _, err := fw.Write([]byte(params.Content)); err != nil {
			return providers.ToolOutput{Content: fmt.Sprintf("error writing new part: %v", err)}
		}
	}

	if err := validateOfficeZip(buf.Bytes()); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("Validation failed. Changes were NOT saved.\nError: %v", err)}
	}

	if err := w.Close(); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error finalizing archive: %v", err)}
	}

	// Determine filename from original
	originalName := resolveFileName(params.FileID, user)
	fileData, err := saveGeneratedFile(buf.Bytes(), originalName, user)
	if err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error saving modified document: %v", err)}
	}

	action := "Updated"
	if !replaced {
		action = "Added"
	}

	return providers.ToolOutput{
		// File:    fileData.ID,
		Content: fmt.Sprintf("%s part '%s'. New file ID: %s (%d bytes) Path: %s", action, params.PartPath, fileData.ID, fileData.Size, fileData.Path),
	}
}

// ── delete_document_part ────────────────────────────────────────────────

func deleteDocumentPartTool(args, user string) providers.ToolOutput {
	var params struct {
		FileID   string `json:"file_id"`
		PartPath string `json:"part_path"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error decoding arguments: %v", err)}
	}

	filePath, err := resolveFilePath(params.FileID, user)
	if err != nil {
		return providers.ToolOutput{Content: err.Error()}
	}

	r, err := zip.OpenReader(filePath)
	if err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error opening document archive: %v", err)}
	}
	defer r.Close()

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	found := false

	for _, f := range r.File {
		if f.Name == params.PartPath {
			found = true
			continue // skip this entry
		}

		fw, err := w.Create(f.Name)
		if err != nil {
			return providers.ToolOutput{Content: fmt.Sprintf("error writing archive: %v", err)}
		}

		rc, err := f.Open()
		if err != nil {
			return providers.ToolOutput{Content: fmt.Sprintf("error reading part: %v", err)}
		}
		if _, err := io.Copy(fw, rc); err != nil {
			rc.Close()
			return providers.ToolOutput{Content: fmt.Sprintf("error copying part: %v", err)}
		}
		rc.Close()
	}

	if !found {
		return providers.ToolOutput{Content: fmt.Sprintf("part '%s' not found in document", params.PartPath)}
	}

	if err := validateOfficeZip(buf.Bytes()); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("Validation failed. Deletion was NOT saved because it corrupts the document.\nError: %v", err)}
	}

	if err := w.Close(); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error finalizing archive: %v", err)}
	}

	originalName := resolveFileName(params.FileID, user)
	fileData, err := saveGeneratedFile(buf.Bytes(), originalName, user)
	if err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error saving modified document: %v", err)}
	}

	return providers.ToolOutput{
		// File:    fileData.ID,
		Content: fmt.Sprintf("Deleted part '%s'. New file ID: %s (%d bytes)", params.PartPath, fileData.ID, fileData.Size),
	}
}

// ── Helpers ─────────────────────────────────────────────────────────────

func validateOfficeZip(data []byte) error {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("invalid zip archive: %v", err)
	}

	files := make(map[string]*zip.File)
	for _, f := range r.File {
		files[f.Name] = f
	}

	for _, f := range r.File {
		name := f.Name
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".xml") && !strings.HasSuffix(lower, ".rels") {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}

		// 1. Verify XML syntax (detects unclosed tags, malformed XML)
		d := xml.NewDecoder(bytes.NewReader(content))
		for {
			_, err := d.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("XML syntax error in '%s': %v", name, err)
			}
		}

		// 2. Validate Content Types References
		if name == "[Content_Types].xml" {
			var types struct {
				Overrides []struct {
					PartName string `xml:"PartName,attr"`
				} `xml:"Override"`
			}
			if err := xml.Unmarshal(content, &types); err == nil {
				for _, o := range types.Overrides {
					target := strings.TrimPrefix(o.PartName, "/")
					if _, ok := files[target]; !ok {
						return fmt.Errorf("reference error in '%s': PartName '%s' not found inside the document", name, o.PartName)
					}
				}
			}
		}

		// 3. Validate Definitions & Relationships
		if strings.HasSuffix(lower, ".rels") {
			var rels struct {
				Rels []struct {
					Target string `xml:"Target,attr"`
				} `xml:"Relationship"`
			}
			if err := xml.Unmarshal(content, &rels); err == nil {
				dir := path.Dir(name)
				baseDir := path.Dir(dir)
				if baseDir == "." {
					baseDir = ""
				} else {
					baseDir += "/"
				}

				for _, r := range rels.Rels {
					if strings.HasPrefix(r.Target, "http://") || strings.HasPrefix(r.Target, "https://") {
						continue
					}
					targetPath := r.Target
					if strings.HasPrefix(targetPath, "/") {
						targetPath = strings.TrimPrefix(targetPath, "/")
					} else {
						targetPath = baseDir + targetPath
					}

					if _, ok := files[targetPath]; !ok {
						clean := strings.ReplaceAll(path.Clean(targetPath), "\\", "/")
						if _, ok2 := files[clean]; !ok2 {
							return fmt.Errorf("reference error in '%s': Target '%s' not found (expected at path '%s')", name, r.Target, targetPath)
						}
					}
				}
			}
		}
	}
	return nil
}

func resolveFilePath(fileID, user string) (string, error) {
	found, err := files.GetByIDs([]string{fileID}, user)
	if err != nil || len(found) == 0 {
		return "", fmt.Errorf("file not found: %s", fileID)
	}
	return found[0].Path, nil
}

func resolveFileName(fileID, user string) string {
	found, err := files.GetByIDs([]string{fileID}, user)
	if err != nil || len(found) == 0 {
		return "document"
	}
	return found[0].Name
}

func buildZip(parts map[string]string) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range parts {
		fw, err := w.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return &buf, nil
}

func saveGeneratedFile(data []byte, fileName, user string) (fs.File, error) {
	uploadDir := path.Join(".", "data", "resources")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return fs.File{}, err
	}

	ext := path.Ext(fileName)
	id := uuid.New().String()
	diskName := id + ext
	filePath := path.Join(uploadDir, diskName)

	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fs.File{}, err
	}

	mimeType := "application/octet-stream"
	switch strings.ToLower(ext) {
	case ".docx":
		mimeType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".pptx":
		mimeType = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".xlsx":
		mimeType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".odt":
		mimeType = "application/vnd.oasis.opendocument.text"
	case ".odp":
		mimeType = "application/vnd.oasis.opendocument.presentation"
	case ".ods":
		mimeType = "application/vnd.oasis.opendocument.spreadsheet"
	}

	now := time.Now().Format(time.RFC3339)
	fileData := fs.File{
		ID:         id,
		Name:       fileName,
		Type:       mimeType,
		Size:       int64(len(data)),
		Path:       filePath,
		User:       user,
		CreatedAt:  now,
		UploadedAt: now,
	}

	if err := files.Save(fileData); err != nil {
		_ = os.Remove(filePath)
		return fs.File{}, err
	}

	return fileData, nil
}

// ── Minimal OpenXML Templates ───────────────────────────────────────────

var minimalTemplates = map[string]map[string]string{
	"docx": {
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:wpc="http://schemas.microsoft.com/office/word/2010/wordprocessingCanvas"
            xmlns:mo="http://schemas.microsoft.com/office/mac/office/2008/main"
            xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"
            xmlns:mv="urn:schemas-microsoft-com:mac:vml"
            xmlns:o="urn:schemas-microsoft-com:office:office"
            xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
            xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math"
            xmlns:v="urn:schemas-microsoft-com:vml"
            xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
            xmlns:w10="urn:schemas-microsoft-com:office:word"
            xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
            xmlns:wne="http://schemas.microsoft.com/office/word/2006/wordml">
  <w:body>
    <w:p>
      <w:r>
        <w:t></w:t>
      </w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/styles.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:docDefaults>
    <w:rPrDefault>
      <w:rPr>
        <w:rFonts w:ascii="Calibri" w:hAnsi="Calibri" w:cs="Calibri"/>
        <w:sz w:val="22"/>
        <w:szCs w:val="22"/>
      </w:rPr>
    </w:rPrDefault>
    <w:pPrDefault>
      <w:pPr>
        <w:spacing w:after="160" w:line="259" w:lineRule="auto"/>
      </w:pPr>
    </w:pPrDefault>
  </w:docDefaults>
  <w:style w:type="paragraph" w:default="1" w:styleId="Normal">
    <w:name w:val="Normal"/>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading1">
    <w:name w:val="heading 1"/>
    <w:basedOn w:val="Normal"/>
    <w:pPr><w:outlineLvl w:val="0"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="32"/><w:szCs w:val="32"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading2">
    <w:name w:val="heading 2"/>
    <w:basedOn w:val="Normal"/>
    <w:pPr><w:outlineLvl w:val="1"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="26"/><w:szCs w:val="26"/></w:rPr>
  </w:style>
</w:styles>`,
	},
	"pptx": {
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
  <Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>
  <Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>
</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`,
		"ppt/_rels/presentation.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
</Relationships>`,
		"ppt/presentation.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
                xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
                xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:sldMasterIdLst>
    <p:sldMasterId id="2147483648" r:id="rId1"/>
  </p:sldMasterIdLst>
  <p:sldIdLst>
    <p:sldId id="256" r:id="rId2"/>
  </p:sldIdLst>
  <p:sldSz cx="12192000" cy="6858000"/>
  <p:notesSz cx="6858000" cy="9144000"/>
</p:presentation>`,
		"ppt/slides/_rels/slide1.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
</Relationships>`,
		"ppt/slides/slide1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
       xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
       xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr>
        <p:cNvPr id="1" name=""/>
        <p:cNvGrpSpPr/>
        <p:nvPr/>
      </p:nvGrpSpPr>
      <p:grpSpPr/>
      <p:sp>
        <p:nvSpPr>
          <p:cNvPr id="2" name="Title 1"/>
          <p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>
          <p:nvPr><p:ph type="ctrTitle"/></p:nvPr>
        </p:nvSpPr>
        <p:spPr>
          <a:xfrm>
            <a:off x="1524000" y="1122363"/>
            <a:ext cx="9144000" cy="2387600"/>
          </a:xfrm>
        </p:spPr>
        <p:txBody>
          <a:bodyPr/>
          <a:lstStyle/>
          <a:p><a:r><a:t>Title</a:t></a:r></a:p>
        </p:txBody>
      </p:sp>
    </p:spTree>
  </p:cSld>
</p:sld>`,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/>
</Relationships>`,
		"ppt/slideLayouts/slideLayout1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
             xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
             xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
             type="blank">
  <p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld>
</p:sldLayout>`,
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
</Relationships>`,
		"ppt/slideMasters/slideMaster1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
             xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
             xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:bg>
      <p:bgPr>
        <a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill>
        <a:effectLst/>
      </p:bgPr>
    </p:bg>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
      <p:grpSpPr/>
    </p:spTree>
  </p:cSld>
  <p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2"
            accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>
  <p:sldLayoutIdLst>
    <p:sldLayoutId id="2147483649" r:id="rId1"/>
  </p:sldLayoutIdLst>
</p:sldMaster>`,
	},
	"xlsx": {
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
  <sheets>
    <sheet name="Sheet1" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>`,
		"xl/worksheets/sheet1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"
           xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheetData/>
</worksheet>`,
		"xl/styles.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <fonts count="1">
    <font><sz val="11"/><name val="Calibri"/></font>
  </fonts>
  <fills count="2">
    <fill><patternFill patternType="none"/></fill>
    <fill><patternFill patternType="gray125"/></fill>
  </fills>
  <borders count="1">
    <border><left/><right/><top/><bottom/><diagonal/></border>
  </borders>
  <cellStyleXfs count="1">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0"/>
  </cellStyleXfs>
  <cellXfs count="1">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>
  </cellXfs>
</styleSheet>`,
		"xl/sharedStrings.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="0" uniqueCount="0"/>`,
	},
}
