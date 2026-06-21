package shareimage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"smarticky/ent"
	"smarticky/ent/mcpimage"
	"smarticky/ent/user"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	ContentTypePNG    = "image/png"
	defaultTheme      = "classic"
	defaultRatio      = "story"
	exportScale       = 2
	maxCanvasSide     = 32760
	maxRenderedRunes  = 500000
	defaultFilePrefix = "smarticky"
)

var (
	markdownImagePattern = regexp.MustCompile(`!\[[^\]]*]\([^)]*\)`)
	markdownLinkPattern  = regexp.MustCompile(`\[([^\]]+)]\([^)]*\)`)
	markdownHeadingMark  = regexp.MustCompile(`(?m)^[#>\s-]*\s*`)
	unsafeFilenameChars  = regexp.MustCompile(`[\\/:*?"<>|\s]+`)
)

type Service struct {
	client  *ent.Client
	dataDir string
	font    *opentype.Font
}

type GenerateInput struct {
	Title   string
	Content string
	Theme   string
	Ratio   string
}

type Generated struct {
	ID          int       `json:"id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	DownloadURL string    `json:"download_url"`
	CreatedAt   time.Time `json:"created_at"`
}

type ImageFile struct {
	ID          int
	Filename    string
	Path        string
	ContentType string
	Size        int64
}

type theme struct {
	background color.RGBA
	surface    color.RGBA
	text       color.RGBA
	muted      color.RGBA
	accent     color.RGBA
	serif      bool
}

type ratio struct {
	width  int
	height int
}

type textBlock struct {
	lines []string
}

type layout struct {
	width           int
	height          int
	margin          int
	surfaceWidth    int
	surfaceHeight   int
	contentX        int
	contentWidth    int
	contentStartY   int
	footerY         int
	titleLines      []string
	bodyBlocks      []textBlock
	titleLineHeight int
	lineHeight      int
	titleGap        int
	paragraphGap    int
}

var themes = map[string]theme{
	"classic": {
		background: mustHex("#f7f6f1"),
		surface:    mustHex("#ffffff"),
		text:       mustHex("#1d1c19"),
		muted:      mustHex("#8b877d"),
		accent:     mustHex("#e8450a"),
		serif:      true,
	},
	"paper": {
		background: mustHex("#eee6d7"),
		surface:    mustHex("#fffaf0"),
		text:       mustHex("#221f19"),
		muted:      mustHex("#8a7862"),
		accent:     mustHex("#bd5c18"),
		serif:      true,
	},
	"night": {
		background: mustHex("#171713"),
		surface:    mustHex("#20201b"),
		text:       mustHex("#f5f1e7"),
		muted:      mustHex("#9b968b"),
		accent:     mustHex("#f4831f"),
		serif:      false,
	},
}

var ratios = map[string]ratio{
	"story":  {width: 1080 * exportScale, height: 1440 * exportScale},
	"square": {width: 1080 * exportScale, height: 1080 * exportScale},
}

func NewService(client *ent.Client, dataDir string) *Service {
	return &Service{
		client:  client,
		dataDir: dataDir,
		font:    loadFont(),
	}
}

func (s *Service) Generate(ctx context.Context, userID int, input GenerateInput) (*Generated, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = "Untitled"
	}

	content := input.Content
	if utf8.RuneCountInString(content) > maxRenderedRunes {
		content = string([]rune(content)[:maxRenderedRunes])
	}

	input.Title = title
	input.Content = content
	data, err := s.Render(input)
	if err != nil {
		return nil, err
	}

	dir := filepath.Join(s.dataDir, "mcp-images", strconv.Itoa(userID))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create image directory: %w", err)
	}

	filename := fmt.Sprintf("%s-%s.png", safeFilePrefix(title), randomSuffix())
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return nil, fmt.Errorf("write image: %w", err)
	}

	row, err := s.client.MCPImage.Create().
		SetFilename(filename).
		SetPath(path).
		SetContentType(ContentTypePNG).
		SetSize(int64(len(data))).
		SetUserID(userID).
		Save(ctx)
	if err != nil {
		_ = os.Remove(path)
		return nil, err
	}

	return &Generated{
		ID:          row.ID,
		Filename:    row.Filename,
		ContentType: row.ContentType,
		Size:        row.Size,
		DownloadURL: "/api/mcp/images/" + strconv.Itoa(row.ID),
		CreatedAt:   row.CreatedAt,
	}, nil
}

func (s *Service) Render(input GenerateInput) ([]byte, error) {
	activeTheme, err := themeFor(input.Theme)
	if err != nil {
		return nil, err
	}
	activeRatio, err := ratioFor(input.Ratio)
	if err != nil {
		return nil, err
	}

	plainTitle := strings.TrimSpace(stripMarkdown(input.Title))
	if plainTitle == "" {
		plainTitle = "Untitled"
	}
	plainContent := strings.TrimSpace(stripMarkdown(input.Content))
	if plainContent == "" {
		plainContent = "No body"
	}

	titleFace, err := s.face(52 * exportScale)
	if err != nil {
		return nil, err
	}
	defer closeFace(titleFace)

	bodySize := 34
	if !activeTheme.serif {
		bodySize = 32
	}
	bodyFace, err := s.face(bodySize * exportScale)
	if err != nil {
		return nil, err
	}
	defer closeFace(bodyFace)

	footerFace, err := s.face(24 * exportScale)
	if err != nil {
		return nil, err
	}
	defer closeFace(footerFace)

	layout := createLayout(activeRatio, activeTheme, titleFace, bodyFace, plainTitle, plainContent)
	if layout.width > maxCanvasSide || layout.height > maxCanvasSide {
		return nil, fmt.Errorf("rendered image is too large: %dx%d", layout.width, layout.height)
	}

	img := image.NewRGBA(image.Rect(0, 0, layout.width, layout.height))
	fillRect(img, 0, 0, layout.width, layout.height, activeTheme.background)
	fillRect(img, layout.margin, layout.margin, layout.surfaceWidth, layout.surfaceHeight, activeTheme.surface)
	fillRect(img, layout.contentX, layout.contentStartY-(42*exportScale), 56*exportScale, 5*exportScale, activeTheme.accent)

	y := layout.contentStartY
	for _, line := range layout.titleLines {
		drawText(img, titleFace, activeTheme.text, layout.contentX, y, line)
		y += layout.titleLineHeight
	}
	y += layout.titleGap

	maxY := layout.height - layout.margin - (128 * exportScale)
	if input.Ratio == "" || input.Ratio == "story" {
		maxY = int(^uint(0) >> 1)
	}
	for _, block := range layout.bodyBlocks {
		for _, line := range block.lines {
			if y > maxY {
				break
			}
			drawText(img, bodyFace, activeTheme.text, layout.contentX, y, line)
			y += layout.lineHeight
		}
		y += layout.paragraphGap
		if y > maxY {
			break
		}
	}

	wordCountLabel := fmt.Sprintf("%d chars", utf8.RuneCountInString(plainContent))
	drawText(img, footerFace, activeTheme.muted, layout.contentX, layout.footerY, "Smarticky")
	rightTextWidth := measureText(footerFace, wordCountLabel)
	drawText(img, footerFace, activeTheme.muted, layout.width-layout.margin-(74*exportScale)-rightTextWidth, layout.footerY, wordCountLabel)

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func (s *Service) GetOwnedImage(ctx context.Context, userID int, id int) (*ImageFile, error) {
	row, err := s.client.MCPImage.Query().
		Where(mcpimage.IDEQ(id), mcpimage.HasUserWith(user.IDEQ(userID))).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return &ImageFile{
		ID:          row.ID,
		Filename:    row.Filename,
		Path:        row.Path,
		ContentType: row.ContentType,
		Size:        row.Size,
	}, nil
}

func (s *Service) face(size int) (font.Face, error) {
	if s.font == nil {
		return basicfont.Face7x13, nil
	}
	return opentype.NewFace(s.font, &opentype.FaceOptions{
		Size:    float64(size),
		DPI:     72,
		Hinting: font.HintingFull,
	})
}

func createLayout(activeRatio ratio, activeTheme theme, titleFace font.Face, bodyFace font.Face, plainTitle string, plainContent string) layout {
	isLongImage := activeRatio.height == ratios["story"].height
	margin := 92 * exportScale
	if isLongImage {
		margin = 108 * exportScale
	}
	surfaceWidth := activeRatio.width - margin*2
	contentX := margin + (74 * exportScale)
	contentWidth := surfaceWidth - (148 * exportScale)
	contentStartY := margin + (112 * exportScale)
	titleLineHeight := 68 * exportScale
	titleGap := 34 * exportScale
	lineHeight := 58 * exportScale
	if activeTheme.serif {
		lineHeight = 62 * exportScale
	}
	paragraphGap := 28 * exportScale

	titleLines := wrapText(titleFace, plainTitle, contentWidth)
	if !isLongImage && len(titleLines) > 3 {
		titleLines = titleLines[:3]
	}

	paragraphs := strings.Split(plainContent, "\n")
	bodyBlocks := make([]textBlock, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		lines := wrapText(bodyFace, paragraph, contentWidth)
		if len(lines) > 0 {
			bodyBlocks = append(bodyBlocks, textBlock{lines: lines})
		}
	}

	y := contentStartY + len(titleLines)*titleLineHeight + titleGap
	fixedMaxY := activeRatio.height - margin - (128 * exportScale)
	for _, block := range bodyBlocks {
		for range block.lines {
			if !isLongImage && y > fixedMaxY {
				break
			}
			y += lineHeight
		}
		y += paragraphGap
		if !isLongImage && y > fixedMaxY {
			break
		}
	}

	footerY := activeRatio.height - margin - (54 * exportScale)
	if isLongImage {
		footerY = maxInt(y+(40*exportScale), activeRatio.height-margin-(54*exportScale))
	}
	height := activeRatio.height
	if isLongImage {
		height = footerY + margin + (54 * exportScale)
	}

	return layout{
		width:           activeRatio.width,
		height:          height,
		margin:          margin,
		surfaceWidth:    surfaceWidth,
		surfaceHeight:   height - margin*2,
		contentX:        contentX,
		contentWidth:    contentWidth,
		contentStartY:   contentStartY,
		footerY:         footerY,
		titleLines:      titleLines,
		bodyBlocks:      bodyBlocks,
		titleLineHeight: titleLineHeight,
		lineHeight:      lineHeight,
		titleGap:        titleGap,
		paragraphGap:    paragraphGap,
	}
}

func wrapText(face font.Face, text string, maxWidth int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	lines := make([]string, 0, 4)
	var line strings.Builder
	for _, r := range text {
		next := line.String() + string(r)
		if measureText(face, next) > maxWidth && line.Len() > 0 {
			lines = append(lines, line.String())
			line.Reset()
			line.WriteRune(r)
			continue
		}
		line.WriteRune(r)
	}
	if line.Len() > 0 {
		lines = append(lines, line.String())
	}
	return lines
}

func measureText(face font.Face, text string) int {
	d := &font.Drawer{Face: face}
	return d.MeasureString(text).Ceil()
}

func drawText(img *image.RGBA, face font.Face, c color.RGBA, x int, y int, text string) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(c),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}

func fillRect(img *image.RGBA, x int, y int, w int, h int, c color.RGBA) {
	draw.Draw(img, image.Rect(x, y, x+w, y+h), image.NewUniform(c), image.Point{}, draw.Src)
}

func stripMarkdown(value string) string {
	value = markdownImagePattern.ReplaceAllString(value, "")
	value = markdownLinkPattern.ReplaceAllString(value, "$1")
	value = markdownHeadingMark.ReplaceAllString(value, "")
	value = strings.NewReplacer("*", "", "_", "", "`", "", "~", "").Replace(value)
	for strings.Contains(value, "\n\n\n") {
		value = strings.ReplaceAll(value, "\n\n\n", "\n\n")
	}
	return value
}

func themeFor(id string) (theme, error) {
	if id == "" {
		id = defaultTheme
	}
	t, ok := themes[id]
	if !ok {
		return theme{}, fmt.Errorf("unsupported theme %q", id)
	}
	return t, nil
}

func ratioFor(id string) (ratio, error) {
	if id == "" {
		id = defaultRatio
	}
	r, ok := ratios[id]
	if !ok {
		return ratio{}, fmt.Errorf("unsupported ratio %q", id)
	}
	return r, nil
}

func safeFilePrefix(title string) string {
	title = strings.TrimSpace(stripMarkdown(title))
	if title == "" {
		title = defaultFilePrefix
	}
	runes := []rune(title)
	if len(runes) > 24 {
		title = string(runes[:24])
	}
	title = unsafeFilenameChars.ReplaceAllString(title, "-")
	title = strings.Trim(title, "-.")
	if title == "" {
		return defaultFilePrefix
	}
	return title
}

func randomSuffix() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(raw[:])
}

func loadFont() *opentype.Font {
	paths := []string{}
	if envPath := strings.TrimSpace(os.Getenv("SMARTICKY_SHARE_FONT")); envPath != "" {
		paths = append(paths, envPath)
	}
	paths = append(paths,
		"/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/TTF/DejaVuSans.ttf",
	)

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		collection, err := opentype.ParseCollection(data)
		if err == nil && collection.NumFonts() > 0 {
			font, err := collection.Font(0)
			if err == nil {
				return font
			}
		}
		font, err := opentype.Parse(data)
		if err == nil {
			return font
		}
	}

	return nil
}

func closeFace(face font.Face) {
	closer, ok := face.(interface{ Close() error })
	if ok {
		_ = closer.Close()
	}
}

func mustHex(value string) color.RGBA {
	c, err := parseHex(value)
	if err != nil {
		panic(err)
	}
	return c
}

func parseHex(value string) (color.RGBA, error) {
	value = strings.TrimPrefix(value, "#")
	if len(value) != 6 {
		return color.RGBA{}, errors.New("hex color must have 6 digits")
	}
	raw, err := hex.DecodeString(value)
	if err != nil {
		return color.RGBA{}, err
	}
	return color.RGBA{R: raw[0], G: raw[1], B: raw[2], A: 255}, nil
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
