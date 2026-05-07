package main

type WindowBounds struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type WindowState struct {
	Bounds   WindowBounds `json:"bounds"`
	TopMost  bool         `json:"topMost"`
	Visible  bool         `json:"visible"`
	Focused  bool         `json:"focused"`
}

func defaultWindowState() WindowState {
	return WindowState{
		Bounds: WindowBounds{
			Width:  860,
			Height: 620,
		},
		TopMost: true,
		Visible: false,
		Focused: false,
	}
}

func (b WindowBounds) Valid() bool {
	return b.Width > 0 && b.Height > 0
}

func (s WindowState) Valid() bool {
	return s.Bounds.Valid()
}

func mergeWindowState(base, loaded WindowState) WindowState {
	if !loaded.Valid() {
		return base
	}
	if loaded.Bounds.Width <= 0 || loaded.Bounds.Height <= 0 {
		loaded.Bounds = base.Bounds
	}
	return loaded
}
