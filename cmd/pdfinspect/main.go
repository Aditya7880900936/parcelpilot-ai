package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
)

func main() {
	dir := "data/pdfs"

	files, err := os.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".pdf") {
			continue
		}

		path := filepath.Join(dir, file.Name())

		text, err := extractText(path)
		if err != nil {
			log.Fatalf("extract %s: %v", file.Name(), err)
		}

		fmt.Printf("\n========================================\n")
		fmt.Printf("FILE: %s\n", file.Name())
		fmt.Printf("========================================\n")
		fmt.Printf("Characters: %d\n\n", len(text))

		preview := text
		if len(preview) > 1000 {
			preview = preview[:1000]
		}

		fmt.Println(preview)
	}
}

func extractText(path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var builder strings.Builder

	for pageNum := 1; pageNum <= r.NumPage(); pageNum++ {
		page := r.Page(pageNum)

		if page.V.IsNull() {
			continue
		}

		text, err := page.GetPlainText(nil)
		if err != nil {
			return "", fmt.Errorf("page %d: %w", pageNum, err)
		}

		builder.WriteString(text)
		builder.WriteString("\n")
	}

	return strings.TrimSpace(builder.String()), nil
}