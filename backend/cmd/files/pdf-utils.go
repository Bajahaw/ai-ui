package files

import (
	"bytes"
	"image/jpeg"
	"math"
	"mime/multipart"
	"net/textproto"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gen2brain/go-fitz"
	nethtml "golang.org/x/net/html"
)

var (
	topRe    = regexp.MustCompile(`top:\s*([-0-9.]+)pt`)
	leftRe   = regexp.MustCompile(`left:\s*([-0-9.]+)pt`)
	widthRe  = regexp.MustCompile(`width:\s*([-0-9.]+)pt`)
	heightRe = regexp.MustCompile(`height:\s*([-0-9.]+)pt`)
	matrixRe = regexp.MustCompile(`transform:matrix\([^,]+,[^,]+,[^,]+,[^,]+,\s*([-0-9.]+)\s*,\s*([-0-9.]+)\s*\)`)
)

func readPDFPages(path string, fileID string) ([]FilePage, error) {
	doc, err := fitz.New(path)
	if err != nil {
		return nil, err
	}
	defer doc.Close()

	var pages []FilePage
	numPage := doc.NumPage()
	for page := 0; page < numPage; page++ {
		text, err := doc.HTML(page, false)
		if err != nil {
			return nil, err
		}

		pageText, err := htmlToText(text)
		if err != nil {
			return nil, err
		}

		pageText += "\n\n--- PAGE " + strconv.Itoa(page+1) + " OF " + strconv.Itoa(numPage) + " ---\n\n"

		pages = append(pages, FilePage{
			ID:         fileID + "-" + strconv.Itoa(page),
			FileID:     fileID,
			PageNumber: page,
			Content:    pageText,
		})
	}
	log.Debug("Extracted pages from PDF", "pages", len(pages))
	return pages, nil
}

// virtualFile satisfies the multipart.File interface
type virtualFile struct {
	*bytes.Reader
}

func (v *virtualFile) Close() error {
	return nil // No-op since it's an in-memory buffer
}

func RenderPDFPageAsImage(path string, pageNumber int, user string) (File, error) {
	doc, err := fitz.New(path)
	if err != nil {
		return File{}, err
	}
	defer doc.Close()

	img, err := doc.Image(pageNumber)
	if err != nil {
		return File{}, err
	}

	var buf bytes.Buffer
	err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	if err != nil {
		return File{}, err
	}

	file := &virtualFile{
		Reader: bytes.NewReader(buf.Bytes()),
	}

	// Mock the FileHeader
	header := &multipart.FileHeader{
		Filename: "screenshot-" + strconv.Itoa(pageNumber) + ".jpg",
		Size:     int64(buf.Len()),
		Header:   make(textproto.MIMEHeader),
	}
	header.Header.Set("Content-Type", "image/jpeg")

	return saveUploadedFile(file, header, user)
}

// func readPDF(path string) (string, error) {
// 	doc, err := fitz.New(path)
// 	if err != nil {
// 		return "", err
// 	}
// 	defer doc.Close()

// 	var totalText strings.Builder
// 	for page := 0; page < doc.NumPage(); page++ {
// 		text, err := doc.HTML(page, false)
// 		if err != nil {
// 			return "", err
// 		}

// 		pageText, err := htmlToText(text)
// 		if err != nil {
// 			return "", err
// 		}

// 		if page > 0 {
// 			totalText.WriteString("\n\n")
// 		}
// 		totalText.WriteString(pageText)
// 	}

// 	result := totalText.String()
// 	log.Debug("Extracted text from PDF", "chars", len(result))
// 	return result, nil
// }

type layoutItem struct {
	x     float64
	y     float64
	order int
	text  string
}

// htmlToText renders HTML content to plain text while attempting to insert image placeholders and preserve some layout based on CSS positioning.
func htmlToText(input string) (string, error) {
	root, err := nethtml.Parse(strings.NewReader(input))
	if err != nil {
		return "", err
	}

	body := findElement(root, "body")
	if body == nil {
		body = root
	}

	items := make([]layoutItem, 0, 64)
	order := 0
	collectLayoutItems(body, &items, &order)

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].y != items[j].y {
			return items[i].y < items[j].y
		}
		if items[i].x != items[j].x {
			return items[i].x < items[j].x
		}
		return items[i].order < items[j].order
	})

	var builder strings.Builder
	for i, item := range items {
		if i > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(item.text)
	}

	return builder.String(), nil
}

func collectLayoutItems(node *nethtml.Node, items *[]layoutItem, order *int) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != nethtml.ElementNode {
			continue
		}

		if child.Data == "img" {
			x, y := extractLayoutPosition(child)
			*items = append(*items, layoutItem{x: x, y: y, order: *order, text: "[image]"})
			*order++
			continue
		}

		text := normalizeText(renderNodeText(child))
		if text == "" {
			collectLayoutItems(child, items, order)
			continue
		}

		x, y := extractLayoutPosition(child)
		if math.IsInf(y, 1) {
			collectLayoutItems(child, items, order)
			continue
		}

		*items = append(*items, layoutItem{x: x, y: y, order: *order, text: text})
		*order++
	}
}

func renderNodeText(node *nethtml.Node) string {
	var builder strings.Builder
	var walk func(*nethtml.Node)
	walk = func(current *nethtml.Node) {
		if current.Type == nethtml.TextNode {
			builder.WriteString(current.Data)
			return
		}

		if current.Type == nethtml.ElementNode && current.Data == "img" {
			builder.WriteString(" [image] ")
			return
		}

		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}

	walk(node)
	return builder.String()
}

func normalizeText(input string) string {
	input = strings.ReplaceAll(input, "\n", " ")
	input = strings.ReplaceAll(input, "\t", " ")
	input = strings.Join(strings.Fields(input), " ")
	return strings.TrimSpace(input)
}

func extractLayoutPosition(node *nethtml.Node) (float64, float64) {
	style := attributeValue(node, "style")
	x := math.Inf(1)
	y := math.Inf(1)

	if matches := topRe.FindStringSubmatch(style); len(matches) == 2 {
		if value, err := strconv.ParseFloat(matches[1], 64); err == nil {
			y = value
		}
	}

	if matches := leftRe.FindStringSubmatch(style); len(matches) == 2 {
		if value, err := strconv.ParseFloat(matches[1], 64); err == nil {
			x = value
		}
	}

	if matches := matrixRe.FindStringSubmatch(style); len(matches) == 3 {
		if x == math.Inf(1) {
			if value, err := strconv.ParseFloat(matches[1], 64); err == nil {
				x = math.Abs(value)
			}
		}
		if y == math.Inf(1) {
			if value, err := strconv.ParseFloat(matches[2], 64); err == nil {
				y = math.Abs(value)
			}
		}
	}

	if x == math.Inf(1) {
		x = 0
	}
	if y == math.Inf(1) {
		y = math.Inf(1)
	}

	return x, y
}

// func extractPageSize(node *nethtml.Node) (float64, float64) {
// 	if node == nil {
// 		return 0, 0
// 	}

// 	style := attributeValue(node, "style")
// 	width := parsePtValue(widthRe, style)
// 	height := parsePtValue(heightRe, style)
// 	if width > 0 && height > 0 {
// 		return width, height
// 	}

// 	for child := node.FirstChild; child != nil; child = child.NextSibling {
// 		if child.Type != nethtml.ElementNode {
// 			continue
// 		}
// 		if width, height := extractPageSize(child); width > 0 && height > 0 {
// 			return width, height
// 		}
// 	}

// 	return 0, 0
// }

// func parsePtValue(pattern *regexp.Regexp, input string) float64 {
// 	matches := pattern.FindStringSubmatch(input)
// 	if len(matches) != 2 {
// 		return 0
// 	}

// 	value, err := strconv.ParseFloat(matches[1], 64)
// 	if err != nil {
// 		return 0
// 	}

// 	return value
// }

func attributeValue(node *nethtml.Node, key string) string {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func findElement(node *nethtml.Node, name string) *nethtml.Node {
	if node == nil {
		return nil
	}
	if node.Type == nethtml.ElementNode && node.Data == name {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := findElement(child, name); found != nil {
			return found
		}
	}
	return nil
}
