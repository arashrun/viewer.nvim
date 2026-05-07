package main

func estimatePixelSize(payload map[string]any) (int, int) {
	const (
		minWidth  = 960
		minHeight = 720
		cellW     = 10
		cellH     = 20
		chromePad = 140
	)

	width := minWidth
	height := minHeight

	if vp, ok := payload["viewport"].(map[string]any); ok {
		if rawW, ok := vp["width"].(float64); ok && rawW > 0 {
			width = int(rawW)*cellW + 40
		}
		if rawH, ok := vp["height"].(float64); ok && rawH > 0 {
			height = int(rawH)*cellH + chromePad
		}
	}

	if width < minWidth {
		width = minWidth
	}
	if height < minHeight {
		height = minHeight
	}

	return width, height
}
