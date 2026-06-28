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
	"strconv"
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

	// Validate that file extension matches the declared format
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(params.FileName)), ".")
	if ext != format {
		return providers.ToolOutput{Content: fmt.Sprintf("error: file_name extension '.%s' does not match format '%s'", ext, format)}
	}

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
		Content: fmt.Sprintf("Created '%s' (%s, %d bytes). File ID: %s Path: /%s", params.FileName, format, fileData.Size, fileData.ID, fileData.Path),
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

	if err := w.Close(); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error finalizing archive: %v", err)}
	}

	if err := validateOfficeZip(buf.Bytes()); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("Validation failed. Changes were NOT saved.\nError: %v", err)}
	}

	if err := updateGeneratedFile(buf.Bytes(), params.FileID, user); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error saving modified document: %v", err)}
	}

	action := "Updated"
	if !replaced {
		action = "Added"
	}

	return providers.ToolOutput{
		Content: fmt.Sprintf("%s part '%s' in file %s (%d bytes).", action, params.PartPath, params.FileID, buf.Len()),
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

	if err := w.Close(); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error finalizing archive: %v", err)}
	}

	if err := validateOfficeZip(buf.Bytes()); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("Validation failed. Deletion was NOT saved because it corrupts the document.\nError: %v", err)}
	}

	if err := updateGeneratedFile(buf.Bytes(), params.FileID, user); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error saving modified document: %v", err)}
	}

	return providers.ToolOutput{
		Content: fmt.Sprintf("Deleted part '%s' from file %s (%d bytes).", params.PartPath, params.FileID, buf.Len()),
	}
}

// ── Helpers ─────────────────────────────────────────────────────────────

// ValidateOfficeZip validates the internal structure of an Office Open XML document.
func ValidateOfficeZip(data []byte) error {
	return validateOfficeZip(data)
}

func validateOfficeZip(data []byte) error {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("invalid zip archive: %v", err)
	}

	files := make(map[string]*zip.File)
	for _, f := range r.File {
		files[f.Name] = f
	}

	overrideTypes := make(map[string]string)        // partPath -> ContentType from [Content_Types].xml
	allRelsIDs := make(map[string]map[string]bool)  // relsPath -> set of Relationship Ids
	partRelRefs := make(map[string]map[string]bool) // partPath -> set of r:id values referenced

	xlParts := make(map[string][]byte)

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
		//    Also collect relationship references (r:id, r:embed, r:link) for forward validation.
		isRelsFile := strings.HasSuffix(lower, ".rels")
		d := xml.NewDecoder(bytes.NewReader(content))
		for {
			tok, err := d.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("XML syntax error in '%s': %v", name, err)
			}
			if !isRelsFile {
				if start, ok := tok.(xml.StartElement); ok {
					for _, attr := range start.Attr {
						if attr.Name.Space == relNamespace &&
							(attr.Name.Local == "id" || attr.Name.Local == "embed" || attr.Name.Local == "link") &&
							attr.Value != "" {
							if partRelRefs[name] == nil {
								partRelRefs[name] = make(map[string]bool)
							}
							partRelRefs[name][attr.Value] = true
						}
					}
				}
			}
		}

		// 2. Validate Content Types References
		if name == "[Content_Types].xml" {
			var types struct {
				Defaults []struct {
					Extension string `xml:"Extension,attr"`
				} `xml:"Default"`
				Overrides []struct {
					PartName    string `xml:"PartName,attr"`
					ContentType string `xml:"ContentType,attr"`
				} `xml:"Override"`
			}
			if err := xml.Unmarshal(content, &types); err == nil {
				overridePaths := make(map[string]bool)
				for _, o := range types.Overrides {
					target := strings.TrimPrefix(o.PartName, "/")
					overridePaths[target] = true
					overrideTypes[target] = o.ContentType
					if _, ok := files[target]; !ok {
						return fmt.Errorf("reference error in '%s': PartName '%s' not found inside the document", name, o.PartName)
					}
				}

				defaultExts := make(map[string]bool)
				for _, d := range types.Defaults {
					defaultExts[strings.ToLower(d.Extension)] = true
				}

				for fName, fObj := range files {
					if fName == "[Content_Types].xml" || fObj.FileInfo().IsDir() {
						continue
					}
					// check if this file is either in overrides or has a default extension
					if !overridePaths[fName] {
						ext := strings.ToLower(path.Ext(fName))
						ext = strings.TrimPrefix(ext, ".")
						if !defaultExts[ext] {
							return fmt.Errorf("invalid part name: '%s' is not defined in [Content_Types].xml and has no matching Default extension", fName)
						}
					}
				}
			}
		}

		// 3. Validate XLSX shared string and style index bounds
		if name == "xl/sharedStrings.xml" || name == "xl/styles.xml" ||
			strings.HasPrefix(name, "xl/worksheets/") {
			// collect into a map for cross-validation after all parts are read
			xlParts[name] = content
		}

		// 4. Validate Definitions & Relationships
		if strings.HasSuffix(lower, ".rels") {
			var rels struct {
				Rels []struct {
					Id     string `xml:"Id,attr"`
					Target string `xml:"Target,attr"`
				} `xml:"Relationship"`
			}
			if err := xml.Unmarshal(content, &rels); err == nil {
				ids := make(map[string]bool)
				dir := path.Dir(name)
				baseDir := path.Dir(dir)
				if baseDir == "." {
					baseDir = ""
				} else {
					baseDir += "/"
				}

				for _, r := range rels.Rels {
					ids[r.Id] = true
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
				allRelsIDs[name] = ids
			}
		}
	}

	// 5. Cross-validate XLSX cell references against shared strings and styles
	if len(xlParts) > 0 {
		if err := validateXLSXSemantics(xlParts); err != nil {
			return err
		}
	}

	// 6. Verify known OOXML parts have the correct content type registered.
	//    The generic <Default Extension="xml"> is not acceptable for parts that
	//    require a specific content type (charts, drawings, worksheets, etc.).
	for fName := range files {
		expected := expectedContentType(fName)
		if expected == "" {
			continue
		}
		actual, ok := overrideTypes[fName]
		if !ok {
			return fmt.Errorf("content type error: part '%s' has no <Override> in [Content_Types].xml — required ContentType '%s'. Fix: add <Override PartName=\"/%s\" ContentType=\"%s\"/> to [Content_Types].xml. Common types: charts='application/vnd.openxmlformats-officedocument.drawingml.chart+xml', drawings='application/vnd.openxmlformats-officedocument.drawing+xml'", fName, expected, fName, expected)
		}
		if actual != expected {
			return fmt.Errorf("content type error: part '%s' is registered with ContentType '%s' but must be '%s'. Fix: change <Override PartName=\"/%s\" ContentType=\"%s\"/> in [Content_Types].xml", fName, actual, expected, fName, expected)
		}
	}

	// 7. Verify forward relationship references (r:id, r:embed, r:link) resolve.
	//    Every r:id in a part must match a <Relationship Id="..."> in the part's
	//    _rels/<name>.rels file. A missing rels file or unmatched id means Excel
	//    will silently discard the referenced object (chart, drawing, image, etc.).
	for partPath, refs := range partRelRefs {
		relsPath := path.Join(path.Dir(partPath), "_rels", path.Base(partPath)+".rels")
		ids, hasRels := allRelsIDs[relsPath]
		if !hasRels {
			return fmt.Errorf("relationship reference error in '%s': part uses r:id but relationship file '%s' does not exist. Fix: create '%s' with a <Relationship> entry for each referenced id. Example: <Relationship Id=\"rId1\" Type=\"http://schemas.openxmlformats.org/officeDocument/2006/relationships/drawing\" Target=\"../drawings/drawing1.xml\"/>", partPath, relsPath, relsPath)
		}
		for ref := range refs {
			if !ids[ref] {
				return fmt.Errorf("relationship reference error in '%s': references r:id='%s' which is not defined in '%s'. Fix: add <Relationship Id=\"%s\" Type=\"...\" Target=\"...\"/> to '%s'", partPath, ref, relsPath, ref, relsPath)
			}
		}
	}

	return nil
}

func validateXLSXSemantics(parts map[string][]byte) error {
	// Count shared strings
	sharedStringCount := 0
	if sst, ok := parts["xl/sharedStrings.xml"]; ok {
		var shared struct {
			Count string `xml:"uniqueCount,attr"`
			Items []struct {
			} `xml:"si"`
		}
		if err := xml.Unmarshal(sst, &shared); err == nil {
			sharedStringCount = len(shared.Items)
			// Also check declared count vs actual
			if shared.Count != "" {
				declared, err := strconv.Atoi(shared.Count)
				if err == nil && declared != sharedStringCount {
					return fmt.Errorf("sharedStrings.xml declares uniqueCount=%d but contains %d entries", declared, sharedStringCount)
				}
			}
		}
	}

	// Count cell style entries (cellXfs)
	cellXfCount := 0
	if styles, ok := parts["xl/styles.xml"]; ok {
		var styleSheet struct {
			CellXfs struct {
				Count string `xml:"count,attr"`
				Xfs   []struct {
				} `xml:"xf"`
			} `xml:"cellXfs"`
		}
		if err := xml.Unmarshal(styles, &styleSheet); err == nil {
			cellXfCount = len(styleSheet.CellXfs.Xfs)
			if styleSheet.CellXfs.Count != "" {
				declared, err := strconv.Atoi(styleSheet.CellXfs.Count)
				if err == nil && declared != cellXfCount {
					return fmt.Errorf("styles.xml declares cellXfs count=%d but contains %d entries", declared, cellXfCount)
				}
			}
		}
	}

	// Validate each worksheet
	for name, content := range parts {
		if !strings.HasPrefix(name, "xl/worksheets/") {
			continue
		}

		var sheet struct {
			Data struct {
				Rows []struct {
					R     string `xml:"r,attr"`
					Cells []struct {
						R string `xml:"r,attr"`
						T string `xml:"t,attr"`
						S string `xml:"s,attr"`
						V string `xml:"v"`
					} `xml:"c"`
				} `xml:"row"`
			} `xml:"sheetData"`
		}
		if err := xml.Unmarshal(content, &sheet); err != nil {
			continue
		}

		lastRow := 0
		for _, row := range sheet.Data.Rows {
			// Check row ordering
			if row.R != "" {
				rowNum, err := strconv.Atoi(row.R)
				if err == nil {
					if rowNum <= lastRow {
						return fmt.Errorf("row ordering error in '%s': row %d appears after row %d", name, rowNum, lastRow)
					}
					lastRow = rowNum
				}
			}

			for _, cell := range row.Cells {
				// Check shared string index bounds
				if cell.T == "s" && cell.V != "" {
					idx, err := strconv.Atoi(cell.V)
					if err == nil && idx >= sharedStringCount {
						return fmt.Errorf("shared string error in '%s' cell %s: references index %d but sharedStrings.xml only has %d entries", name, cell.R, idx, sharedStringCount)
					}
				}

				// Check style index bounds
				if cell.S != "" {
					idx, err := strconv.Atoi(cell.S)
					if err == nil && cellXfCount > 0 && idx >= cellXfCount {
						return fmt.Errorf("style error in '%s' cell %s: references style index %d but styles.xml only has %d cellXf entries", name, cell.R, idx, cellXfCount)
					}
				}
			}
		}
	}

	return nil
}

// relNamespace is the XML namespace for relationship references in OOXML parts.
var relNamespace = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"

// expectedContentType returns the required content type for known OOXML part paths.
// Returns "" if the path has no specific requirement (may use the Default extension mapping).
func expectedContentType(partPath string) string {
	exact := map[string]string{
		"xl/workbook.xml":      "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml",
		"xl/styles.xml":        "application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml",
		"xl/sharedStrings.xml": "application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml",
		"word/document.xml":    "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml",
		"word/styles.xml":      "application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml",
		"ppt/presentation.xml": "application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml",
	}
	if ct, ok := exact[partPath]; ok {
		return ct
	}
	lower := strings.ToLower(partPath)
	prefixes := []struct {
		prefix string
		ct     string
	}{
		{"xl/worksheets/", "application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"},
		{"xl/drawings/", "application/vnd.openxmlformats-officedocument.drawing+xml"},
		{"xl/charts/", "application/vnd.openxmlformats-officedocument.drawingml.chart+xml"},
		{"ppt/slides/", "application/vnd.openxmlformats-officedocument.presentationml.slide+xml"},
		{"ppt/slidelayouts/", "application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"},
		{"ppt/slidemasters/", "application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"},
		{"ppt/theme/", "application/vnd.openxmlformats-officedocument.theme+xml"},
	}
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p.prefix) && strings.HasSuffix(lower, ".xml") {
			return p.ct
		}
	}
	return ""
}

func resolveFilePath(fileID, user string) (string, error) {
	found, err := files.GetByIDs([]string{fileID}, user)
	if err != nil || len(found) == 0 {
		return "", fmt.Errorf("file not found: %s", fileID)
	}
	return found[0].Path, nil
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
	case ".png":
		mimeType = "image/png"
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".webp":
		mimeType = "image/webp"
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

func updateGeneratedFile(data []byte, fileID, user string) error {
	found, err := files.GetByIDs([]string{fileID}, user)
	if err != nil || len(found) == 0 {
		return fmt.Errorf("file not found: %s", fileID)
	}

	if err := os.WriteFile(found[0].Path, data, 0o644); err != nil {
		return fmt.Errorf("error overwriting file: %v", err)
	}

	if err := files.UpdateSize(fileID, user, int64(len(data))); err != nil {
		return fmt.Errorf("error updating file record: %v", err)
	}

	return nil
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
  <Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>
</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`,
		"ppt/_rels/presentation.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>
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
  <p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>
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
  <p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>
</p:sldLayout>`,
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>
</Relationships>`,

		"ppt/theme/theme1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?> <a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office Theme"><a:themeElements><a:clrScheme name="Office"><a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1><a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1><a:dk2><a:srgbClr val="44546A"/></a:dk2><a:lt2><a:srgbClr val="E7E6E6"/></a:lt2><a:accent1><a:srgbClr val="4472C4"/></a:accent1><a:accent2><a:srgbClr val="ED7D31"/></a:accent2><a:accent3><a:srgbClr val="A5A5A5"/></a:accent3><a:accent4><a:srgbClr val="FFC000"/></a:accent4><a:accent5><a:srgbClr val="5B9BD5"/></a:accent5><a:accent6><a:srgbClr val="70AD47"/></a:accent6><a:hlink><a:srgbClr val="0563C1"/></a:hlink><a:folHlink><a:srgbClr val="954F72"/></a:folHlink></a:clrScheme><a:fontScheme name="Office"><a:majorFont><a:latin typeface="Calibri Light" panose="020F0302020204030204"/><a:ea typeface=""/><a:cs typeface=""/><a:font script="Jpan" typeface="游ゴシック Light"/><a:font script="Hang" typeface="맑은 고딕"/><a:font script="Hans" typeface="等线 Light"/><a:font script="Hant" typeface="新細明體"/><a:font script="Arab" typeface="Times New Roman"/><a:font script="Hebr" typeface="Times New Roman"/><a:font script="Thai" typeface="Angsana New"/><a:font script="Ethi" typeface="Nyala"/><a:font script="Beng" typeface="Vrinda"/><a:font script="Gujr" typeface="Shruti"/><a:font script="Khmr" typeface="MoolBoran"/><a:font script="Knda" typeface="Tunga"/><a:font script="Guru" typeface="Raavi"/><a:font script="Cans" typeface="Euphemia"/><a:font script="Cher" typeface="Plantagenet Cherokee"/><a:font script="Yiii" typeface="Microsoft Yi Baiti"/><a:font script="Tibt" typeface="Microsoft Himalaya"/><a:font script="Thaa" typeface="MV Boli"/><a:font script="Deva" typeface="Mangal"/><a:font script="Telu" typeface="Gautami"/><a:font script="Taml" typeface="Latha"/><a:font script="Syrc" typeface="Estrangelo Edessa"/><a:font script="Orya" typeface="Kalinga"/><a:font script="Mlym" typeface="Kartika"/><a:font script="Laoo" typeface="DokChampa"/><a:font script="Sinh" typeface="Iskoola Pota"/><a:font script="Mong" typeface="Mongolian Baiti"/><a:font script="Viet" typeface="Times New Roman"/><a:font script="Uigh" typeface="Microsoft Uighur"/><a:font script="Geor" typeface="Sylfaen"/><a:font script="Armn" typeface="Arial"/><a:font script="Bugi" typeface="Leelawadee UI"/><a:font script="Bopo" typeface="Microsoft JhengHei"/><a:font script="Java" typeface="Javanese Text"/><a:font script="Lisu" typeface="Segoe UI"/><a:font script="Mymr" typeface="Myanmar Text"/><a:font script="Nkoo" typeface="Ebrima"/><a:font script="Olck" typeface="Nirmala UI"/><a:font script="Osma" typeface="Ebrima"/><a:font script="Phag" typeface="Phagspa"/><a:font script="Syrn" typeface="Estrangelo Edessa"/><a:font script="Syrj" typeface="Estrangelo Edessa"/><a:font script="Syre" typeface="Estrangelo Edessa"/><a:font script="Sora" typeface="Nirmala UI"/><a:font script="Tale" typeface="Microsoft Tai Le"/><a:font script="Talu" typeface="Microsoft New Tai Lue"/><a:font script="Tfng" typeface="Ebrima"/></a:majorFont><a:minorFont><a:latin typeface="Calibri" panose="020F0502020204030204"/><a:ea typeface=""/><a:cs typeface=""/><a:font script="Jpan" typeface="游ゴシック"/><a:font script="Hang" typeface="맑은 고딕"/><a:font script="Hans" typeface="等线"/><a:font script="Hant" typeface="新細明體"/><a:font script="Arab" typeface="Arial"/><a:font script="Hebr" typeface="Arial"/><a:font script="Thai" typeface="Cordia New"/><a:font script="Ethi" typeface="Nyala"/><a:font script="Beng" typeface="Vrinda"/><a:font script="Gujr" typeface="Shruti"/><a:font script="Khmr" typeface="DaunPenh"/><a:font script="Knda" typeface="Tunga"/><a:font script="Guru" typeface="Raavi"/><a:font script="Cans" typeface="Euphemia"/><a:font script="Cher" typeface="Plantagenet Cherokee"/><a:font script="Yiii" typeface="Microsoft Yi Baiti"/><a:font script="Tibt" typeface="Microsoft Himalaya"/><a:font script="Thaa" typeface="MV Boli"/><a:font script="Deva" typeface="Mangal"/><a:font script="Telu" typeface="Gautami"/><a:font script="Taml" typeface="Latha"/><a:font script="Syrc" typeface="Estrangelo Edessa"/><a:font script="Orya" typeface="Kalinga"/><a:font script="Mlym" typeface="Kartika"/><a:font script="Laoo" typeface="DokChampa"/><a:font script="Sinh" typeface="Iskoola Pota"/><a:font script="Mong" typeface="Mongolian Baiti"/><a:font script="Viet" typeface="Arial"/><a:font script="Uigh" typeface="Microsoft Uighur"/><a:font script="Geor" typeface="Sylfaen"/><a:font script="Armn" typeface="Arial"/><a:font script="Bugi" typeface="Leelawadee UI"/><a:font script="Bopo" typeface="Microsoft JhengHei"/><a:font script="Java" typeface="Javanese Text"/><a:font script="Lisu" typeface="Segoe UI"/><a:font script="Mymr" typeface="Myanmar Text"/><a:font script="Nkoo" typeface="Ebrima"/><a:font script="Olck" typeface="Nirmala UI"/><a:font script="Osma" typeface="Ebrima"/><a:font script="Phag" typeface="Phagspa"/><a:font script="Syrn" typeface="Estrangelo Edessa"/><a:font script="Syrj" typeface="Estrangelo Edessa"/><a:font script="Syre" typeface="Estrangelo Edessa"/><a:font script="Sora" typeface="Nirmala UI"/><a:font script="Tale" typeface="Microsoft Tai Le"/><a:font script="Talu" typeface="Microsoft New Tai Lue"/><a:font script="Tfng" typeface="Ebrima"/></a:minorFont></a:fontScheme><a:fmtScheme name="Office"><a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:gradFill rotWithShape="1"><a:gsLst><a:gs pos="0"><a:schemeClr val="phClr"><a:lumMod val="110000"/><a:satMod val="105000"/><a:tint val="67000"/></a:schemeClr></a:gs><a:gs pos="50000"><a:schemeClr val="phClr"><a:lumMod val="105000"/><a:satMod val="103000"/><a:tint val="73000"/></a:schemeClr></a:gs><a:gs pos="100000"><a:schemeClr val="phClr"><a:lumMod val="105000"/><a:satMod val="109000"/><a:tint val="81000"/></a:schemeClr></a:gs></a:gsLst><a:lin ang="5400000" scaled="0"/></a:gradFill><a:gradFill rotWithShape="1"><a:gsLst><a:gs pos="0"><a:schemeClr val="phClr"><a:satMod val="103000"/><a:lumMod val="102000"/><a:tint val="94000"/></a:schemeClr></a:gs><a:gs pos="50000"><a:schemeClr val="phClr"><a:satMod val="110000"/><a:lumMod val="100000"/><a:shade val="100000"/></a:schemeClr></a:gs><a:gs pos="100000"><a:schemeClr val="phClr"><a:lumMod val="99000"/><a:satMod val="120000"/><a:shade val="78000"/></a:schemeClr></a:gs></a:gsLst><a:lin ang="5400000" scaled="0"/></a:gradFill></a:fillStyleLst><a:lnStyleLst><a:ln w="6350" cap="flat" cmpd="sng" algn="ctr"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:prstDash val="solid"/><a:miter lim="800000"/></a:ln><a:ln w="12700" cap="flat" cmpd="sng" algn="ctr"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:prstDash val="solid"/><a:miter lim="800000"/></a:ln><a:ln w="19050" cap="flat" cmpd="sng" algn="ctr"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:prstDash val="solid"/><a:miter lim="800000"/></a:ln></a:lnStyleLst><a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst><a:outerShdw blurRad="57150" dist="19050" dir="5400000" algn="ctr" rotWithShape="0"><a:srgbClr val="000000"><a:alpha val="63000"/></a:srgbClr></a:outerShdw></a:effectLst></a:effectStyle></a:effectStyleLst><a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"><a:tint val="95000"/><a:satMod val="170000"/></a:schemeClr></a:solidFill><a:gradFill rotWithShape="1"><a:gsLst><a:gs pos="0"><a:schemeClr val="phClr"><a:tint val="93000"/><a:satMod val="150000"/><a:shade val="98000"/><a:lumMod val="102000"/></a:schemeClr></a:gs><a:gs pos="50000"><a:schemeClr val="phClr"><a:tint val="98000"/><a:satMod val="130000"/><a:shade val="90000"/><a:lumMod val="103000"/></a:schemeClr></a:gs><a:gs pos="100000"><a:schemeClr val="phClr"><a:shade val="63000"/><a:satMod val="120000"/></a:schemeClr></a:gs></a:gsLst><a:lin ang="5400000" scaled="0"/></a:gradFill></a:bgFillStyleLst></a:fmtScheme></a:themeElements><a:objectDefaults/><a:extraClrSchemeLst/><a:extLst><a:ext uri="{05A4C25C-085E-4340-85A3-A5531E510DB2}"><thm15:themeFamily xmlns:thm15="http://schemas.microsoft.com/office/thememl/2012/main" name="Office Theme" id="{62F939B6-93AF-4DB8-9C6B-D6C7DFDC589F}" vid="{4A3C46E8-61CC-4603-A589-7422A47A8E4A}"/></a:ext></a:extLst></a:theme>`,
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
