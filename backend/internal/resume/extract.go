package resume

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
)

// extractText turns an uploaded resume into plain text. It supports PDF, DOCX
// and plain text; anything else is treated as UTF-8 text.
func extractText(filename string, data []byte) (string, error) {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".pdf"):
		return extractPDF(data)
	case strings.HasSuffix(lower, ".docx"):
		return extractDOCX(data)
	default:
		return string(data), nil
	}
}

func extractPDF(data []byte) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open pdf: %w", err)
	}
	var buf bytes.Buffer
	plain, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("read pdf text: %w", err)
	}
	if _, err := io.Copy(&buf, plain); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// extractDOCX reads word/document.xml from the .docx zip and concatenates all
// text nodes. Paragraph boundaries become newlines.
func extractDOCX(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open docx: %w", err)
	}
	var docXML io.ReadCloser
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			docXML, err = f.Open()
			if err != nil {
				return "", err
			}
			break
		}
	}
	if docXML == nil {
		return "", fmt.Errorf("docx: word/document.xml not found")
	}
	defer docXML.Close()

	dec := xml.NewDecoder(docXML)
	var sb strings.Builder
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.CharData:
			sb.Write(t)
		case xml.EndElement:
			// End of a paragraph -> newline for readability.
			if t.Name.Local == "p" {
				sb.WriteByte('\n')
			}
		}
	}
	return sb.String(), nil
}
