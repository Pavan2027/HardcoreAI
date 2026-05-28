package ingestion

import (
	"fmt"
	"regexp"
	"strings"
)

type Chunk struct {
	DocumentID      int
	ChunkIndex      int
	ChunkText       string
	SectionTitle    string
	SubsectionTitle string
	Peripheral      string
	RegisterName    string
	PageNumber      int
	TokenCount      int
}

type Chunker struct {
	ChunkSize    int // target chars per chunk
	ChunkOverlap int // overlap chars between chunks
}

func NewChunker() *Chunker {
	return &Chunker{
		ChunkSize:    1500, // ~375 tokens
		ChunkOverlap: 200,
	}
}

// Known STM32 peripherals to tag chunks with
var knownPeripherals = []string{
	"GPIO", "USART", "UART", "SPI", "I2C", "I2S", "TIM", "ADC", "DAC",
	"DMA", "RCC", "NVIC", "EXTI", "FLASH", "CRC", "RTC", "IWDG", "WWDG",
	"USB", "ETH", "CAN", "SDIO", "DCMI", "FPU", "MPU", "PWR", "SYSCFG",
}

// Regex for section headers like "2.3.1 GPIO Configuration"
var sectionHeaderRegex = regexp.MustCompile(`(?m)^(\d+(\.\d+)*)\s+([A-Z][^\n]{3,60})$`)

// Regex for register names like GPIO_MODER, USART_BRR
var registerNameRegex = regexp.MustCompile(`\b([A-Z]{2,10}_[A-Z][A-Z0-9_]{2,20})\b`)

// Regex to strip embedded page markers like "DS8626 Rev 12 5/206"
var pageMarkerRegex = regexp.MustCompile(`DS\d+\s+Rev\s+\d+\s*\d+/\d+`)

func (c *Chunker) ChunkDocument(doc *ParsedDocument) []Chunk {
	fmt.Printf("✂️  Chunking %d pages...\n", len(doc.Pages))

	var chunks []Chunk
	chunkIndex := 0

	for _, page := range doc.Pages {
		// Clean the page text
		cleanText := cleanPageText(page.Text)
		if len(cleanText) < 50 {
			continue // skip near-empty pages
		}

		// Split page into chunks
		pageChunks := c.splitIntoChunks(cleanText, page.PageNumber, chunkIndex)
		chunks = append(chunks, pageChunks...)
		chunkIndex += len(pageChunks)
	}

	fmt.Printf("✅ Created %d chunks\n", len(chunks))
	return chunks
}

func (c *Chunker) splitIntoChunks(text string, pageNum, startIndex int) []Chunk {
	var chunks []Chunk

	// If page text is smaller than chunk size, treat as one chunk
	if len(text) <= c.ChunkSize {
		chunk := c.buildChunk(text, pageNum, startIndex)
		return append(chunks, chunk)
	}

	// Split into overlapping chunks
	start := 0
	localIndex := 0
	for start < len(text) {
		end := start + c.ChunkSize
		if end > len(text) {
			end = len(text)
		}

		// Try to break at a sentence boundary
		if end < len(text) {
			lastPeriod := strings.LastIndex(text[start:end], ". ")
			if lastPeriod > c.ChunkSize/2 {
				end = start + lastPeriod + 2
			}
		}

		chunkText := strings.TrimSpace(text[start:end])
		if len(chunkText) > 50 {
			chunk := c.buildChunk(chunkText, pageNum, startIndex+localIndex)
			chunks = append(chunks, chunk)
			localIndex++
		}

		// Move forward with overlap
		// Move forward with overlap
		newStart := end - c.ChunkOverlap
		if newStart <= start {
			// prevent infinite loop
			newStart = end
		}
		start = newStart
		if start >= len(text) {
			break
		}
	}

	return chunks
}

func (c *Chunker) buildChunk(text string, pageNum, index int) Chunk {
	return Chunk{
		ChunkIndex:   index,
		ChunkText:    text,
		PageNumber:   pageNum,
		TokenCount:   estimateTokens(text),
		SectionTitle: extractSectionTitle(text),
		Peripheral:   detectPeripheral(text),
		RegisterName: extractRegisterName(text),
	}
}

func cleanPageText(text string) string {
	// Remove embedded page markers
	text = pageMarkerRegex.ReplaceAllString(text, "")
	// Collapse multiple newlines
	multiNewline := regexp.MustCompile(`\n{3,}`)
	text = multiNewline.ReplaceAllString(text, "\n\n")
	// Collapse multiple spaces
	multiSpace := regexp.MustCompile(`[ \t]{2,}`)
	text = multiSpace.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func extractSectionTitle(text string) string {
	matches := sectionHeaderRegex.FindStringSubmatch(text)
	if len(matches) >= 4 {
		return strings.TrimSpace(matches[3])
	}
	return ""
}

func detectPeripheral(text string) string {
	upper := strings.ToUpper(text)
	for _, p := range knownPeripherals {
		if strings.Contains(upper, p) {
			return p
		}
	}
	return ""
}

func extractRegisterName(text string) string {
	matches := registerNameRegex.FindStringSubmatch(text)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

func estimateTokens(text string) int {
	// Rough estimate: 1 token ≈ 4 characters
	return len(text) / 4
}
