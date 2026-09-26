// Element-level locating built on the DOM domain: resolve a CSS selector to a
// node, scroll it into view and read its box model, so tools can click, focus
// or crop an element instead of guessing coordinates.
//
// Geometry notes: DOM.getBoxModel reports coordinates in the visual viewport
// (already scroll-adjusted, verified against getBoundingClientRect). Mouse
// events and screenshot clips need different frames, so ElementBox keeps both:
//   - X/Y/Width/Height are viewport coordinates and are used for
//     Input.dispatchMouseEvent;
//   - PageX/PageY add the scroll offset (document coordinates) and are used for
//     the screenshot clip.

package browser

import (
	"context"
	"fmt"
	"time"
)

// defaultElementTimeout bounds how long ElementBox waits for a selector.
const defaultElementTimeout = 5 * time.Second

// ElementBox describes the geometry of a resolved element.
type ElementBox struct {
	NodeID int
	X      float64 // viewport left (CSS px)
	Y      float64 // viewport top (CSS px)
	Width  float64
	Height float64
	PageX  float64 // page left (CSS px)
	PageY  float64 // page top (CSS px)
}

// Center returns the click point of the element in viewport coordinates.
func (b *ElementBox) Center() (float64, float64) {
	return b.X + b.Width/2, b.Y + b.Height/2
}

// Clip returns a Page.captureScreenshot clip covering the element.
func (b *ElementBox) Clip() map[string]interface{} {
	return map[string]interface{}{
		"x":      b.PageX,
		"y":      b.PageY,
		"width":  b.Width,
		"height": b.Height,
		"scale":  1,
	}
}

// jsonFloat reads a numeric JSON value, returning 0 for anything else.
func jsonFloat(v interface{}) float64 {
	f, _ := v.(float64)
	return f
}

// QuerySelectorNodeID resolves selector to a DOM node id. It returns an error
// when the selector matches no element.
func (c *CDPClient) QuerySelectorNodeID(ctx context.Context, selector string) (int, error) {
	if selector == "" {
		return 0, fmt.Errorf("selector must not be empty")
	}
	root, err := c.GetDocument(ctx)
	if err != nil {
		return 0, fmt.Errorf("cannot get DOM document: %w", err)
	}
	result, err := c.Call(ctx, "DOM.querySelector", map[string]interface{}{
		"nodeId":   root,
		"selector": selector,
	})
	if err != nil {
		return 0, fmt.Errorf("cannot query selector %q: %w", selector, err)
	}
	nodeID := int(jsonFloat(result["nodeId"]))
	if nodeID == 0 {
		return 0, fmt.Errorf("no element matches selector %q", selector)
	}
	return nodeID, nil
}

// ScrollIntoViewIfNeeded makes the node visible in the viewport.
func (c *CDPClient) ScrollIntoViewIfNeeded(ctx context.Context, nodeID int) error {
	if _, err := c.Call(ctx, "DOM.scrollIntoViewIfNeeded", map[string]interface{}{
		"nodeId": nodeID,
	}); err != nil {
		return fmt.Errorf("cannot scroll node %d into view: %w", nodeID, err)
	}
	return nil
}

// ElementBox waits for selector (bounded by timeout), scrolls the element into
// view and returns its geometry.
func (c *CDPClient) ElementBox(ctx context.Context, selector string, timeout time.Duration) (*ElementBox, error) {
	if timeout <= 0 {
		timeout = defaultElementTimeout
	}
	if err := c.WaitForSelector(ctx, selector, timeout); err != nil {
		return nil, err
	}

	nodeID, err := c.QuerySelectorNodeID(ctx, selector)
	if err != nil {
		return nil, err
	}
	if err := c.ScrollIntoViewIfNeeded(ctx, nodeID); err != nil {
		return nil, err
	}

	box, err := c.Call(ctx, "DOM.getBoxModel", map[string]interface{}{"nodeId": nodeID})
	if err != nil {
		return nil, fmt.Errorf("cannot get box model for %q: %w", selector, err)
	}
	model, ok := box["model"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected box model for %q", selector)
	}
	quad, ok := model["border"].([]interface{})
	if !ok || len(quad) < 8 {
		return nil, fmt.Errorf("unexpected border quad for %q", selector)
	}

	xs := []float64{jsonFloat(quad[0]), jsonFloat(quad[2]), jsonFloat(quad[4]), jsonFloat(quad[6])}
	ys := []float64{jsonFloat(quad[1]), jsonFloat(quad[3]), jsonFloat(quad[5]), jsonFloat(quad[7])}
	left, right := minFloat(xs), maxFloat(xs)
	top, bottom := minFloat(ys), maxFloat(ys)
	if right-left <= 0 || bottom-top <= 0 {
		return nil, fmt.Errorf("element %q has no visible box", selector)
	}

	// The box model is viewport-relative: mouse events use it as-is, while the
	// screenshot clip is expressed in document coordinates.
	offsetX, offsetY := c.viewportScrollOffset(ctx)

	return &ElementBox{
		NodeID: nodeID,
		X:      left,
		Y:      top,
		Width:  right - left,
		Height: bottom - top,
		PageX:  left + offsetX,
		PageY:  top + offsetY,
	}, nil
}

// viewportScrollOffset returns the scroll offset of the visual viewport in CSS
// pixels (pageX/pageY from Page.getLayoutMetrics). It returns 0,0 when the
// metrics are unavailable so callers fall back to page coordinates.
func (c *CDPClient) viewportScrollOffset(ctx context.Context) (float64, float64) {
	metrics, err := c.Call(ctx, "Page.getLayoutMetrics", nil)
	if err != nil {
		return 0, 0
	}
	for _, name := range []string{"cssVisualViewport", "cssLayoutViewport", "visualViewport", "layoutViewport"} {
		if vp, ok := metrics[name].(map[string]interface{}); ok {
			return jsonFloat(vp["pageX"]), jsonFloat(vp["pageY"])
		}
	}
	return 0, 0
}

// CaptureScreenshotClip captures the given clip region of the page and returns
// base64-encoded JPEG data.
func (c *CDPClient) CaptureScreenshotClip(ctx context.Context, quality int, clip map[string]interface{}) (string, error) {
	if clip == nil {
		return "", fmt.Errorf("clip must not be nil")
	}
	result, err := c.Call(ctx, "Page.captureScreenshot", map[string]interface{}{
		"format":                "jpeg",
		"quality":               quality,
		"clip":                  clip,
		"captureBeyondViewport": true,
	})
	if err != nil {
		return "", err
	}
	data, _ := result["data"].(string)
	return data, nil
}

// minFloat returns the smallest value of xs (xs must not be empty).
func minFloat(xs []float64) float64 {
	m := xs[0]
	for _, v := range xs[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

// maxFloat returns the largest value of xs (xs must not be empty).
func maxFloat(xs []float64) float64 {
	m := xs[0]
	for _, v := range xs[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
